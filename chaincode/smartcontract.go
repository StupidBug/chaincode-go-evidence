package chaincode

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/accumulator/merkle"
	"github.com/consensys/gnark/std/hash/mimc"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// EvidenceContract
type EvidenceContract struct {
	contractapi.Contract
}

// EvidenceList
type EvidenceList struct {
	Records []EvidenceRecord `json:"records"`
}

// EvidenceLockType
type EvidenceLockType string

const (
	SideChainLock EvidenceLockType = "sidechain"
	MainChainLock EvidenceLockType = "mainchain"
)

// EvidenceLock
type EvidenceLock struct {
	Hash           string            `json:"Hash"`
	ClientID       string            `json:"clientID"`
	Timeout        int64             `json:"Timeout"`
	DataHash       string            `json:"DataHash"`
	LockType       EvidenceLockType  `json:"LockType"`
	EvidenceData   string            `json:"EvidenceData"`
	EvidenceID     string            `json:"EvidenceID"`
	Status         string            `json:"Status"`
	CreateTime     int64             `json:"CreateTime"`
	UpdateTime     int64             `json:"UpdateTime"`
	ChainID        string            `json:"ChainID"`
	AdditionalInfo map[string]string `json:"AdditionalInfo"`
}

// EvidenceRecord
type EvidenceRecord struct {
	EvidenceID     string            `json:"EvidenceID"`
	DataHash       string            `json:"DataHash"`
	EvidenceData   string            `json:"EvidenceData"`
	Status         string            `json:"Status"`
	CreateTime     int64             `json:"CreateTime"`
	UpdateTime     int64             `json:"UpdateTime"`
	ChainID        string            `json:"ChainID"`
	AdditionalInfo map[string]string `json:"AdditionalInfo"`
}

// Initialize
func (e *EvidenceContract) Initialize(ctx contractapi.TransactionContextInterface) error {
	list := EvidenceList{Records: make([]EvidenceRecord, 0)}
	listBytes, _ := json.Marshal(list)
	return ctx.GetStub().PutState("evidence_list", listBytes)
}

// CreateSideChainLock
func (e *EvidenceContract) CreateSideChainLock(
	ctx contractapi.TransactionContextInterface,
	clientID string, seed int64, timeout int,
	dataHash, evidenceID, chainID string, // 移除evidenceData参数
) (string, error) {

	list, err := e.getEvidenceList(ctx)
	if err != nil {
		return "", err
	}

	// 验证数据在侧链已存在
	exists := false
	for _, record := range list.Records {
		if record.ChainID == chainID && record.DataHash == dataHash {
			exists = true
			break
		}
	}
	if !exists {
		return "", fmt.Errorf("data not found on sidechain")
	}

	// 生成随机数R和哈希
	r := strconv.FormatInt(seed, 10)
	hashR := fmt.Sprintf("%x", sha256.Sum256([]byte(r)))

	// 创建锁
	lock := EvidenceLock{
		Hash:       hashR,
		ClientID:   clientID,
		Timeout:    time.Now().Add(time.Duration(timeout) * time.Second).UnixMilli(),
		DataHash:   dataHash,
		LockType:   SideChainLock,
		EvidenceID: evidenceID,
		Status:     "ACTIVE",
		CreateTime: time.Now().UnixMilli(),
		ChainID:    chainID,
	}

	lockKey := fmt.Sprintf("lock_%s_%s", clientID, hashR)
	return hashR, e.saveLock(ctx, lockKey, lock)
}

// CreateMainChainLock
func (e *EvidenceContract) CreateMainChainLock(
	ctx contractapi.TransactionContextInterface,
	hashValue, clientID string, timeout int,
	dataHash, evidenceData, evidenceID, chainID string,
	sidechainChainID string,
) error {

	list, err := e.getEvidenceList(ctx)
	if err != nil {
		return err
	}

	// 验证主链数据唯一性（只检查主链）
	for _, record := range list.Records {
		if record.ChainID == chainID && record.DataHash == dataHash {
			return fmt.Errorf("data already exists on main chain")
		}
	}

	// 验证数据哈希匹配
	if fmt.Sprintf("%x", sha256.Sum256([]byte(evidenceData))) != dataHash {
		return fmt.Errorf("evidence data hash mismatch")
	}

	// 创建主链存证记录（状态初始为PENDING_SYNC）
	record := EvidenceRecord{
		EvidenceID:   evidenceID,
		DataHash:     dataHash,
		EvidenceData: evidenceData,
		Status:       "PENDING_SYNC", // 新增同步状态
		CreateTime:   time.Now().UnixMilli(),
		UpdateTime:   time.Now().UnixMilli(),
		ChainID:      chainID,
		AdditionalInfo: map[string]string{
			"sidechain_chain_id": sidechainChainID, // 记录侧链来源
		},
	}

	// 创建主链锁
	lock := EvidenceLock{
		Hash:       hashValue,
		ClientID:   clientID,
		Timeout:    time.Now().Add(time.Duration(timeout) * time.Second).UnixMilli(),
		DataHash:   dataHash,
		LockType:   MainChainLock,
		EvidenceID: evidenceID,
		Status:     "ACTIVE",
		CreateTime: time.Now().UnixMilli(),
		ChainID:    chainID,
	}

	// 保存数据
	list.Records = append(list.Records, record)
	if err := e.saveEvidenceList(ctx, list); err != nil {
		return err
	}

	lockKey := fmt.Sprintf("lock_%s_%s", clientID, hashValue)
	return e.saveLock(ctx, lockKey, lock)
}

// VerifyAndUnlock
func (e *EvidenceContract) VerifyAndUnlock(
	ctx contractapi.TransactionContextInterface,
	lockKey, preimage string,
) error {

	lock, err := e.getLock(ctx, lockKey)
	if err != nil {
		return err
	}

	// 验证哈希
	if fmt.Sprintf("%x", sha256.Sum256([]byte(preimage))) != lock.Hash {
		return fmt.Errorf("invalid preimage")
	}

	list, err := e.getEvidenceList(ctx)
	if err != nil {
		return err
	}

	// 更新主链存证状态
	for i, record := range list.Records {
		if record.EvidenceID == lock.EvidenceID {
			// 同步侧链附加信息
			if sidechainID, ok := record.AdditionalInfo["sidechain_chain_id"]; ok {
				list.Records[i].AdditionalInfo["synced_chain"] = sidechainID
			}

			list.Records[i].Status = "CONFIRMED"
			list.Records[i].UpdateTime = time.Now().UnixMilli()
			break
		}
	}

	// 删除锁并更新状态
	ctx.GetStub().DelState(lockKey)
	return e.saveEvidenceList(ctx, list)
}

func (e *EvidenceContract) SubmitSideChainData(
	ctx contractapi.TransactionContextInterface,
	dataHash, evidenceData, evidenceID, chainID string,
) error {

	list, err := e.getEvidenceList(ctx)
	if err != nil {
		return err
	}

	// 检查侧链数据唯一性
	for _, record := range list.Records {
		if record.ChainID == chainID && record.DataHash == dataHash {
			return fmt.Errorf("data already exists on this chain")
		}
	}

	record := EvidenceRecord{
		EvidenceID:     evidenceID,
		DataHash:       dataHash,
		EvidenceData:   evidenceData,
		Status:         "AVAILABLE", // 新增可用状态
		CreateTime:     time.Now().UnixMilli(),
		UpdateTime:     time.Now().UnixMilli(),
		ChainID:        chainID,
		AdditionalInfo: make(map[string]string),
	}

	list.Records = append(list.Records, record)
	return e.saveEvidenceList(ctx, list)
}

func (e *EvidenceContract) ConfirmSync(ctx contractapi.TransactionContextInterface, evidenceID string) error {
	list, _ := e.getEvidenceList(ctx)
	for i, record := range list.Records {
		if record.EvidenceID == evidenceID && record.Status == "CONFIRMED" {
			newR, _ := generateRandom()
			newHashC := fmt.Sprintf("%x", sha256.Sum256([]byte(record.DataHash+newR)))

			list.Records[i].DataHash = newHashC
			list.Records[i].AdditionalInfo["latest_r_hash"] =
				fmt.Sprintf("%x", sha256.Sum256([]byte(newR)))
			list.Records[i].UpdateTime = time.Now().UnixMilli()
			return e.saveEvidenceList(ctx, list)
		}
	}
	return fmt.Errorf("record not confirmed")
}

// TimeoutRollback
func (e *EvidenceContract) TimeoutRollback(
	ctx contractapi.TransactionContextInterface,
	lockKey string,
) error {

	lock, err := e.getLock(ctx, lockKey)
	if err != nil {
		return err
	}

	// 检查超时
	if time.Now().UnixMilli() < lock.Timeout {
		return fmt.Errorf("lock not expired")
	}

	// 更新记录状态
	list, err := e.getEvidenceList(ctx)
	if err != nil {
		return err
	}

	for i, record := range list.Records {
		if record.EvidenceID == lock.EvidenceID {
			list.Records[i].Status = "EXPIRED"
			list.Records[i].UpdateTime = time.Now().UnixMilli()
			break
		}
	}

	// 删除锁并更新状态
	ctx.GetStub().DelState(lockKey)
	return e.saveEvidenceList(ctx, list)
}

func (e *EvidenceContract) getEvidenceList(ctx contractapi.TransactionContextInterface) (EvidenceList, error) {
	listBytes, _ := ctx.GetStub().GetState("evidence_list")
	var list EvidenceList
	if listBytes != nil {
		json.Unmarshal(listBytes, &list)
	}
	return list, nil
}

func (e *EvidenceContract) saveEvidenceList(ctx contractapi.TransactionContextInterface, list EvidenceList) error {
	listBytes, _ := json.Marshal(list)
	return ctx.GetStub().PutState("evidence_list", listBytes)
}

func (e *EvidenceContract) saveLock(ctx contractapi.TransactionContextInterface, key string, lock EvidenceLock) error {
	lockBytes, _ := json.Marshal(lock)
	return ctx.GetStub().PutState(key, lockBytes)
}

func (e *EvidenceContract) getLock(ctx contractapi.TransactionContextInterface, key string) (EvidenceLock, error) {
	lockBytes, _ := ctx.GetStub().GetState(key)
	var lock EvidenceLock
	if lockBytes == nil {
		return lock, fmt.Errorf("lock not found")
	}
	json.Unmarshal(lockBytes, &lock)
	return lock, nil
}

func generateRandom() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", bytes), nil
}

// 输出参数结构体
type StorageProofOutput struct {
	OldStateRoot string
	BatchRoot    string
	NewStateRoot string
	Evidence     []EvidenceRecord // 添加新的账户状态字段
	Proof        interface{}      // 使用interface{}来存储proof
	Vk           interface{}      // 使用interface{}来存储vk
}

type storageCircuit struct {
	// 公开输入
	OldStateRoot   frontend.Variable `gnark:",public"` // 旧状态根
	RootHash       frontend.Variable `gnark:",public"` // 批次根
	FinalStateRoot frontend.Variable `gnark:",public"` // 最终状态根

	// 私有输入
	Evidence []frontend.Variable // 存证数据
	Path     []frontend.Variable // 默克尔路径
	Helper   []frontend.Variable // 默克尔路径辅助数据
}

// 定义电路
func (circuit *storageCircuit) Define(curveID ecc.ID, api frontend.API) error {
	hFunc, err := mimc.NewMiMC("seed", curveID, api)
	if err != nil {
		return err
	}

	// 验证批次根的默克尔路径，证明存证在批次中
	merkle.VerifyProof(api, hFunc, circuit.RootHash, circuit.Path, circuit.Helper)

	// 计算新的状态根
	stateHasher, _ := mimc.NewMiMC("seed", curveID, api)

	// 根据旧状态根和批次根的哈希值计算新的状态根
	stateHasher.Reset()
	stateHasher.Write(circuit.OldStateRoot)
	stateHasher.Write(circuit.RootHash)
	computedNewRoot := stateHasher.Sum()

	// 验证计算的新状态根是否与预期一致，证明批次存证的正确性
	api.AssertIsEqual(circuit.FinalStateRoot, computedNewRoot)

	for i := 0; i < len(circuit.Evidence); i++ {
		// 验证每一个存证数据不能为空
		api.AssertIsDifferent(circuit.Evidence[i], api.Constant(0))
	}

	return nil
}

// VerifyStorageProof verifies the storage proof without saving it
func (e *EvidenceContract) VerifyStorageProof(ctx contractapi.TransactionContextInterface, storageProofStr string) error {
	// 反序列化输入
	var output StorageProofOutput
	err := json.Unmarshal([]byte(storageProofStr), &output)
	if err != nil {
		return fmt.Errorf("failed to deserialize proof: %v", err)
	}

	publicWitness := &storageCircuit{
		OldStateRoot:   frontend.Value(output.OldStateRoot),
		RootHash:       frontend.Value(output.BatchRoot),
		FinalStateRoot: frontend.Value(output.NewStateRoot),
	}

	proof, ok := output.Proof.(groth16.Proof)
	if !ok {
		return fmt.Errorf("invalid proof type")
	}

	vk, ok := output.Vk.(groth16.VerifyingKey)
	if !ok {
		return fmt.Errorf("invalid vk type")
	}

	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		return fmt.Errorf("proof verification failed: %v", err)
	}
	// 验证证明
	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		return fmt.Errorf("proof verification failed: %v", err)
	}
	return nil
}

// CreateStorageProof creates a new storage proof and adds it to the world state
func (e *EvidenceContract) CreateStorageProof(ctx contractapi.TransactionContextInterface, id string, storageProofStr string) error {
	// Deserialize the input storage proof string
	var output StorageProofOutput
	err := json.Unmarshal([]byte(storageProofStr), &output)
	if err != nil {
		return fmt.Errorf("failed to deserialize storage proof: %v", err)
	}

	// 保存存证
	err = e.SaveEvidences(ctx, output.Evidence)
	if err != nil {
		return fmt.Errorf("save evidence failed: %v", err)
	}

	output.Evidence = nil
	proofStr, err := json.Marshal(output)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState("output"+id, proofStr)
}

// GetAllStorageProof returns all storage proofs found in the world state
func (e *EvidenceContract) GetAllStorageProof(ctx contractapi.TransactionContextInterface) (string, error) {
	// Range query with empty string for startKey and endKey does an
	// open-ended query of all storage proofs in the chaincode namespace
	resultsIterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return "", err
	}
	defer resultsIterator.Close()

	var storageProofs []*StorageProofOutput
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return "", err
		}

		// 只处理以"output"开头的key
		if !strings.HasPrefix(queryResponse.Key, "output") {
			continue
		}

		var storageProof StorageProofOutput
		err = json.Unmarshal(queryResponse.Value, &storageProof)
		if err != nil {
			return "", fmt.Errorf("unmarshal storage proof output [key: %s] failed [str: %s]: %w", queryResponse.Key, string(queryResponse.Value), err)
		}
		storageProofs = append(storageProofs, &storageProof)
	}
	proofs, err := json.Marshal(&storageProofs)
	if err != nil {
		return "", err
	}

	return string(proofs), nil
}

// VerifySaveStorageProof verifies the storage proof and if valid, saves it to the world state
func (e *EvidenceContract) VerifySaveStorageProof(ctx contractapi.TransactionContextInterface, id string, storageProofStr string) error {
	// First verify the storage proof
	err := e.VerifyStorageProof(ctx, storageProofStr)
	if err != nil {
		return fmt.Errorf("storage proof verification failed: %v", err)
	}

	// If verification passes, save the storage proof
	err = e.CreateStorageProof(ctx, id, storageProofStr)
	if err != nil {
		return fmt.Errorf("failed to save storage proof: %v", err)
	}

	return nil
}

// SaveEvidences 将新的存证添加到现有存证中并存入区块链
func (e *EvidenceContract) SaveEvidences(ctx contractapi.TransactionContextInterface, datas []EvidenceRecord) error {
	// 读取现有存证
	evidenceList, err := e.getEvidenceList(ctx)
	if err != nil {
		return fmt.Errorf("failed to read evidences: %v", err)
	}
	lastID := -1
	if len(evidenceList.Records) != 0 {
		idStr := evidenceList.Records[len(evidenceList.Records)-1].EvidenceID
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return fmt.Errorf("last id not valid: %w", err)
		}
		lastID = id
	}
	if strconv.Itoa(lastID+1) != datas[0].EvidenceID {
		return fmt.Errorf("last id not match")
	}
	evidenceList.Records = append(evidenceList.Records, datas...)
	return e.saveEvidenceList(ctx, evidenceList)
}

// GetAllEvidences 返回所有存储在anyKey中的存证
func (e *EvidenceContract) GetAllEvidences(ctx contractapi.TransactionContextInterface) ([]string, error) {
	evidenceList, err := e.getEvidenceList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read evidences: %v", err)
	}
	if len(evidenceList.Records) == 0 {
		return []string{}, nil
	}

	evidenceDataList := make([]string, 0, len(evidenceList.Records))
	for _, record := range evidenceList.Records {
		evidenceDataList = append(evidenceDataList, record.EvidenceData)
	}
	return evidenceDataList, nil
}

// 序列化的输出结构体
type SerializedStorageProofOutput struct {
	OldStateRoot string           `json:"old_state_root"`
	BatchRoot    string           `json:"batch_root"`
	NewStateRoot string           `json:"new_state_root"`
	Evidence     []EvidenceRecord `json:"evidence"`
	ProofData    string           `json:"proof"` // base64编码的proof数据
	VkData       string           `json:"vk"`    // base64编码的vk数据
}

// 反序列化为StorageProofOutput
func (p *StorageProofOutput) UnmarshalJSON(data []byte) error {
	// 解析序列化的数据
	var serialized SerializedStorageProofOutput
	if err := json.Unmarshal(data, &serialized); err != nil {
		return err
	}

	// 解码base64数据
	proofBytes, err := base64.StdEncoding.DecodeString(serialized.ProofData)
	if err != nil {
		return fmt.Errorf("failed to decode proof data: %v", err)
	}

	vkBytes, err := base64.StdEncoding.DecodeString(serialized.VkData)
	if err != nil {
		return fmt.Errorf("failed to decode vk data: %v", err)
	}

	// 初始化proof和vk
	proof := groth16.NewProof(ecc.BN254)
	vk := groth16.NewVerifyingKey(ecc.BN254)

	// 反序列化proof和vk
	if _, err := proof.ReadFrom(bytes.NewReader(proofBytes)); err != nil {
		return fmt.Errorf("failed to read proof: %v", err)
	}

	if _, err := vk.ReadFrom(bytes.NewReader(vkBytes)); err != nil {
		return fmt.Errorf("failed to read vk: %v", err)
	}

	// 设置字段值
	p.OldStateRoot = serialized.OldStateRoot
	p.BatchRoot = serialized.BatchRoot
	p.NewStateRoot = serialized.NewStateRoot
	p.Evidence = serialized.Evidence
	p.Proof = proof
	p.Vk = vk

	return nil
}

// 序列化StorageProofOutput
func (p StorageProofOutput) MarshalJSON() ([]byte, error) {
	// 创建缓冲区
	proofBuf := new(bytes.Buffer)
	vkBuf := new(bytes.Buffer)

	// 类型断言
	proof, ok := p.Proof.(io.WriterTo)
	if !ok {
		return nil, fmt.Errorf("proof does not implement WriterTo")
	}

	vk, ok := p.Vk.(io.WriterTo)
	if !ok {
		return nil, fmt.Errorf("vk does not implement WriterTo")
	}

	// 序列化proof和vk
	if _, err := proof.WriteTo(proofBuf); err != nil {
		return nil, fmt.Errorf("failed to write proof: %v", err)
	}

	if _, err := vk.WriteTo(vkBuf); err != nil {
		return nil, fmt.Errorf("failed to write vk: %v", err)
	}

	// 创建序列化结构体
	serialized := SerializedStorageProofOutput{
		OldStateRoot: p.OldStateRoot,
		BatchRoot:    p.BatchRoot,
		NewStateRoot: p.NewStateRoot,
		Evidence:     p.Evidence,
		ProofData:    base64.StdEncoding.EncodeToString(proofBuf.Bytes()),
		VkData:       base64.StdEncoding.EncodeToString(vkBuf.Bytes()),
	}

	// 序列化为JSON
	return json.Marshal(serialized)
}
