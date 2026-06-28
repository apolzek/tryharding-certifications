---
title: Hyperledger Fabric Certified Practitioner (HFCP)
markmap:
  colorFreezeLevel: 2
---

<!--
  ACRONYM: HFCP = "Hyperledger Fabric Certified Practitioner", a Linux
  Foundation certification (exam based on Hyperledger Fabric v2.5).
  Domains and weights below are the official exam blueprint.
  Source: training.linuxfoundation.org/certification/hyperledger-fabric-certified-practitioner-hfcp/
-->

# HFCP

## Fundamentals of Blockchain (16%)
### Core blockchain concepts
- Distributed ledgers
  - Blockchain (hash-chained blocks) + world state
  - Immutability via SHA-256 block hashing
  - Permissioned vs permissionless ledgers
- Smart contracts
  - Chaincode packaged as smart contract
  - Deterministic execution requirement
- Consensus
  - Endorsement -> ordering -> validation phases
  - Pluggable ordering (Raft CFT)
  - No proof-of-work / no mining in Fabric
- Hyperledger Fabric model
  - Permissioned network with identities (X.509)
  - Modular architecture (peers, orderers, CAs)
  - Channels for data partitioning/privacy
- Business benefits
  - Provenance, finality, no central intermediary
  - Confidentiality via channels + private data

## Hyperledger Fabric Networks (36%)
### Network architecture and operations
- Network structure
  - Organizations (Orgs) and consortium definition
  - Peers, orderers, clients, CAs as components
  - Channel artifacts: genesis block, channel.tx
- Transaction flow
  - Propose -> endorse -> order -> validate -> commit
  - Read/write set (RW-set) generation
  - MVCC conflict check at validation
- Ordering service
  - Raft consensus (etcd/raft, leader+followers)
  - Orderer nodes batch txs into blocks (BatchTimeout/BatchSize)
  - Channel-based ordering (no system channel in 2.5, channel participation API)
- Peers and world state storage
  - World state DB: LevelDB (default) or CouchDB
  - CouchDB enables rich JSON queries (indexes)
  - Block storage on peer filesystem (ledger)
  - Anchor peers for cross-org gossip
- Network creation
  - cryptogen / Fabric CA for identities
  - configtxgen to build genesis block + channel tx
  - configtx.yaml (Organizations/Profiles/Policies)
  - peer channel create / peer channel join
- Membership Service Providers (MSP)
  - X.509 certificate-based identities
  - Roles: member, admin, client, peer, orderer
  - Local MSP (node) vs channel MSP
  - CA root/intermediate certs, NodeOUs (config.yaml)
- Maintenance and operation
  - peer node start; orderer process
  - Logging: FABRIC_LOGGING_SPEC
  - Operations service metrics (Prometheus endpoint)
  - Backup/restore of ledger and CouchDB
- Channel-related operations
  - peer channel update for config updates
  - Channel config: capabilities (V2_5), batch params
  - Adding an org via configtxlator + signed config update
- Design and deployment of production networks
  - TLS enabled (mutual TLS between nodes)
  - HSM (PKCS#11) for key protection
  - Kubernetes / Docker Compose deployment
  - High availability: multi-orderer Raft (>=3, odd number)

## Smart Contracts (24%)
### Chaincode design and ledger interaction
- Design and implementation
  - Languages: Go, Node.js (JavaScript/TS), Java
  - contractapi (Go) / fabric-contract-api (Node)
  - Init and transaction functions
- Chaincode lifecycle (Fabric v2.x)
  - peer lifecycle chaincode package
  - peer lifecycle chaincode install
  - peer lifecycle chaincode approveformyorg
  - peer lifecycle chaincode checkcommitreadiness
  - peer lifecycle chaincode commit
  - Decentralized governance: per-org approval
- Reading and modifying the ledger state
  - GetState(key) / PutState(key, value)
  - DelState(key)
  - peer chaincode invoke -C channel -n cc -c '{...}'
- Executing queries on the ledger
  - GetStateByRange (range query)
  - GetQueryResult (CouchDB rich query, Mango selector)
  - GetHistoryForKey (key history)
  - peer chaincode query
- Private data and private data collections
  - collections_config.json definition
  - GetPrivateData / PutPrivateData
  - Private data hash on channel ledger; data via gossip
  - Implicit org collections (_implicit_org_MSPID)
- State-based endorsement policies (SBE)
  - SetStateValidationParameter per key
  - Signature policies: AND/OR(Org.member)
  - --signature-policy flag on commit

## Client Applications (24%)
### Application development against Fabric
- Gateway model
  - Fabric Gateway SDKs: fabric-gateway (Go/Node/Java)
  - Connection via gRPC to peer gateway (port 7051)
  - Identity + Signer (X.509 cert + private key)
- Peer gateway service
  - Embedded in peer (Fabric v2.4+), simplifies submit
  - Handles endorsement collection + ordering submit
  - gateway.Connect(...) -> Network -> Contract
- Smart contract invocation
  - contract.SubmitTransaction(name, args) (write)
  - contract.EvaluateTransaction(name, args) (query)
  - Endorsement gathering per endorsement policy
- Chaincode and block eventing
  - Contract event listeners (ChaincodeEvents)
  - Block event listeners (BlockEvents)
  - Checkpointing for event replay (startBlock)
- Offline signing
  - Separate sign step (private key off-host/HSM)
  - Endorse -> sign digest -> Submit flow
  - Useful for cold-key custody
