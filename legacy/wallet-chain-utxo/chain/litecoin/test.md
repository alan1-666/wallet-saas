# Bitcoin rpc api test

## 1.get fee
- request
```
grpcurl -plaintext -d '{
  "chain": "Litecoin",
  "coin": "LTC",
  "network": "mainet"
}' 127.0.0.1:8289 dapplink.utxo.WalletUtxoService.getFee
```
- response
```
{
  "code": "SUCCESS",
  "msg": "get litcoin fee success",
  "best_fee": "0.00011985",
  "best_fee_sat": "11985",
  "slow_fee": "",
  "normal_fee": "",
  "fast_fee": ""
}
```

## 2.get account
- request
```
grpcurl -plaintext -d '{
  "chain": "Litecoin",
  "network": "mainet",
  "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
}' 127.0.0.1:8289 dapplink.utxo.WalletUtxoService.getAccount
```

- response
```
{
  "code": "SUCCESS",
  "msg": "Get litecoin account info success",
  "network": "",
  "balance": "0.00297156"
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
  "chain": "Litecoin",
  "coin": "LTC",
  "network": "mainnet",
  "hash": "7e592bfe5d3cabbd6f03e73076b15b9f9b8d672235de8a8c5045268093f5d80f"
}' 127.0.0.1:8289 dapplink.utxo.WalletUtxoService.getTxByHash
```

- response
```
{
  "code": "SUCCESS",
  "msg": "get transaction detail success",
  "tx": {
    "hash": "7e592bfe5d3cabbd6f03e73076b15b9f9b8d672235de8a8c5045268093f5d80f",
    "index": 0,
    "froms": [
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      }
    ],
    "tos": [
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      },
      {
        "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
      }
    ],
    "fee": "0.00001262",
    "status": "Success",
    "values": [
      {
        "value": "38690"
      },
      {
        "value": "0.00297156"
      }
    ],
    "type": 0,
    "height": "2784926",
    "brc20_address": "",
    "datetime": "1730642079000"
  }
}
```

## 5.get address utxo
- request
```
grpcurl -plaintext -d '{
  "chain": "Litecoin",
  "network": "mainnet",
  "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq"
}' 127.0.0.1:8289 dapplink.utxo.WalletUtxoService.getUnspentOutputs
```
- response
```
{
  "code": "SUCCESS",
  "msg": "get ltc utxo success",
  "unspent_outputs": [
    {
      "tx_id": "7e592bfe5d3cabbd6f03e73076b15b9f9b8d672235de8a8c5045268093f5d80f",
      "tx_hash_big_endian": "",
      "tx_output_n": "1",
      "script": "",
      "height": "2784926",
      "block_time": "1730642079",
      "address": "MVRidwRCeGpmfDoeZ2wc1LtjMEuQ5gSxnq",
      "unspent_amount": "0.00297156",
      "value_hex": "",
      "confirmations": "0",
      "index": "1"
    }
  ]
}
```