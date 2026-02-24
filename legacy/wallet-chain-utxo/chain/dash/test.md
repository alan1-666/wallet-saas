# Bitcoin rpc api test

## 1.get fee
- request
```
grpcurl -plaintext -d '{
  "chain": "Dash",
  "coin": "Dash",
  "network": "mainet"
}' 127.0.0.1:8289 dapplink.utxo.WalletUtxoService.getFee
```
- response
```
{
  "code": "SUCCESS",
  "msg": "get dash fee success",
  "best_fee": "0.00003733",
  "best_fee_sat": "3733",
  "slow_fee": "",
  "normal_fee": "",
  "fast_fee": ""
}
```

## 2.get account
- request
```
grpcurl -plaintext -d '{
  "chain": "Dash",
  "network": "mainet",
  "address": "Xrh6ouWeDBR8nLo95du2RLq8PbPZMe5vks"
}' 127.0.0.1:8289 dapplink.utxo.WalletUtxoService.getAccount
```

- response
```
{
  "code": "SUCCESS",
  "msg": "Get dash account info success",
  "network": "",
  "balance": "0.000505"
}
```

## 3.get tx by address
- request
```

```

- response
```

```

## 4.get tx by hash
- request
```
grpcurl -plaintext -d '{
  "chain": "Dash",
  "coin": "Dash",
  "network": "mainnet",
  "hash": "90bede613044bbaf76bdcc35089bf5a14151a33fa03b22632fb285b0186d27d8"
}' 127.0.0.1:8289 dapplink.utxo.WalletUtxoService.getTxByHash
```

- response
```
{
  "code": "SUCCESS",
  "msg": "get transaction detail success",
  "tx": {
    "hash": "90bede613044bbaf76bdcc35089bf5a14151a33fa03b22632fb285b0186d27d8",
    "index": 0,
    "froms": [
      {
        "address": "Xrh6ouWeDBR8nLo95du2RLq8PbPZMe5vks"
      }
    ],
    "tos": [
      {
        "address": "Xrh6ouWeDBR8nLo95du2RLq8PbPZMe5vks"
      },
      {
        "address": "XxitcnX5hsHQKsG3hoD61Ujagtt1N6vDE3"
      }
    ],
    "fee": "0.00000454",
    "status": "Success",
    "values": [
      {
        "value": "1000.995"
      },
      {
        "value": "94925.61925541"
      }
    ],
    "type": 0,
    "height": "2166470",
    "brc20_address": "",
    "datetime": "1730714431000"
  }
}
```

## 5.get address utxo
- request
```
grpcurl -plaintext -d '{
  "chain": "Dash",
  "network": "mainnet",
  "address": "Xmz9uAZriHqZjan7PUajSjY2sUBwMkYoFe"
}' 127.0.0.1:8289 dapplink.utxo.WalletUtxoService.getUnspentOutputs
```
- response
```
{
  "code": "SUCCESS",
  "msg": "get dash utxo success",
  "unspent_outputs": [
    {
      "tx_id": "d5cde9a826c1d08fdbaec7a88d597d530a76bc42958b4eaba38192d7151b2525",
      "tx_hash_big_endian": "",
      "tx_output_n": "0",
      "script": "",
      "height": "2166484",
      "block_time": "1730715685",
      "address": "Xmz9uAZriHqZjan7PUajSjY2sUBwMkYoFe",
      "unspent_amount": "116.13452641",
      "value_hex": "",
      "confirmations": "0",
      "index": "0"
    }
  ]
}
```