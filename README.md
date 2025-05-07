# 版本
fabric2.2.7
go1.23.0

# 通道创建
cd fabric-samples/test-network
./network.sh up createChannel -c mychannel -ca

# 链码编译
go mod tidy
go mod vendor
go build ./...


# 部署链码
./network.sh deployCC \
  -ccn evidence_chaincode \
  -ccp /home/wudelun/files/fabric/scripts/fabric-samples/evidence-persistance/chaincode-go-evidence \
  -ccl go
./network.sh deployCC \
  -ccn evidence_chaincode \
  -ccp /home/tjz/go_workspace/src/fabric_2.2.7/fabric/scripts/fabric-samples/evidence-persistance/chaincode-go-evidence \
  -ccl go


# 设置环境变量
peer变量：
echo 'export PATH=$PATH:~/files/fabric/scripts/fabric-samples/bin' >> ~/.bashrc
source ~/.bashrc
或者
export PATH=$PATH:~/files/fabric/scripts/fabric-samples/bin

# PEER变量
export CORE_PEER_LOCALMSPID="Org1MSP"
export CORE_PEER_ADDRESS=peer0.org1.example.com:7051

# 其他变量
*WSL:*
export FABRIC_CFG_PATH=$HOME/files/fabric/scripts/fabric-samples/config

*THU-2080:*
export FABRIC_CFG_PATH=/home/tjz/go_workspace/src/fabric_2.2.7/fabric/scripts/fabric-samples/config

export CORE_PEER_MSPCONFIGPATH="${PWD}/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp"


# 调用实例 

1. Initialize

peer chaincode invoke \
  -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --tls \
  --cafile "/home/tjz/go_workspace/src/fabric_2.2.7/fabric/scripts/fabric-samples/test-network/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" \
  -C mychannel \
  -n evidence \
  --peerAddresses localhost:7051 \
  --tlsRootCertFiles " /home/tjz/go_workspace/src/fabric_2.2.7/fabric/scripts/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
  --peerAddresses localhost:9051 \
  --tlsRootCertFiles " /home/tjz/go_workspace/src/fabric_2.2.7/fabric/scripts/fabric-samples/test-network/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" \
  -c '{"function":"Initialize","Args":[]}'


2. SubmitSideChainData

peer chaincode invoke \
  -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --tls \
  --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" \
  -C mychannel \
  -n evidence \
  --peerAddresses localhost:7051 \
  --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
  --peerAddresses localhost:9051 \
  --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" \
  -c '{
    "function":"SubmitSideChainData",
    "Args":[
      "0x9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", 
      "{\"content\":\"test data\"}",  
      "EVID-20231101-001",  
      "sidechain-01"  
    ]
  }'


3. CreateSideChainLock

peer chaincode invoke \
  -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --tls \
  --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" \
  -C mychannel \
  -n evidence \
  --peerAddresses localhost:7051 \
  --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
  --peerAddresses localhost:9051 \
  --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" \
  -c '{
    "function":"CreateSideChainLock",
    "Args":[
      "client-org1-admin",  
      "1700000000000",      
      "600",              
      "0x9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",  
      "EVID-20231101-001",  
      "sidechain-01"       
    ]
  }'


4. CreateMainChainLock

peer chaincode invoke \
  -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --tls \
  --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" \
  -C mychannel \
  -n evidence \
  --peerAddresses localhost:7051 \
  --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
  --peerAddresses localhost:9051 \
  --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" \
  -c '{
    "function":"CreateMainChainLock",
    "Args":[
      "0x2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
      "client-org2-admin",  
      "1200",             
      "0x9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", 
      "{\"content\":\"main data\"}", 
      "MAIN-EVID-001",      
      "mainchain-01",     
      "sidechain-01"       
    ]
  }'


5. VerifyAndUnlock

peer chaincode invoke \
  -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --tls \
  --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" \
  -C mychannel \
  -n evidence \
  --peerAddresses localhost:7051 \
  --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
  --peerAddresses localhost:9051 \
  --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" \
  -c '{
    "function":"VerifyAndUnlock",
    "Args":[
      "lock_client-org1-admin_0x2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", 
      "secretPassword123"  
    ]
  }'


6. TimeoutRollback

peer chaincode invoke \
  -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --tls \
  --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" \
  -C mychannel \
  -n evidence \
  --peerAddresses localhost:7051 \
  --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
  --peerAddresses localhost:9051 \
  --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" \
  -c '{
    "function":"TimeoutRollback",
    "Args":["lock_client-org2-admin_0x2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"]
  }'


7. ConfirmSync

peer chaincode invoke \
  -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --tls \
  --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" \
  -C mychannel \
  -n evidence \
  --peerAddresses localhost:7051 \
  --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
  --peerAddresses localhost:9051 \
  --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" \
  -c '{
    "function":"ConfirmSync",
    "Args":["MAIN-EVID-001"]
  }'


参数生成工具

# 生成数据哈希
data="test content"
dataHash=$(echo -n "$data" | sha256sum | awk '{print "0x"$1}')

# 生成随机数
random=$(openssl rand -hex 32)

# 生成时间戳
timestamp=$(date +%s%3N)

# 生成锁键名
clientID="client-org1-admin"
lockKey="lock_${clientID}_${dataHash}"

验证操作完整流程
提交侧链数据 → 2. 创建侧链锁 → 3. 创建主链锁 → 4. 验证解锁 → 5. 确认同步