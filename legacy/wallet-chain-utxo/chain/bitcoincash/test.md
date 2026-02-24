# Bitcoin rpc api test

## 1.get fee
- request
```
grpcurl -plaintext -d '{
  "chain": "BitcoinCash",
  "coin": "BCH",
  "network": "mainet"
}' 127.0.0.1:8289 dapplink.utxo.WalletUtxoService.getFee
```
- response
```
{
  "code": "SUCCESS",
  "msg": "get bch fee success",
  "best_fee": "0.00004815",
  "best_fee_sat": "4815",
  "slow_fee": "",
  "normal_fee": "",
  "fast_fee": ""
}
```


## 2.get account
- request
```
grpcurl -plaintext -d '{
  "chain": "BitcoinCash",
  "network": "mainet",
  "address": "pqwjhsy0qsj8e6drfm5gua6lcdlqagczjq7vh5kak6"
}' 127.0.0.1:8289 dapplink.utxo.WalletUtxoService.getAccount
```

- response
```
{
  "code": "SUCCESS",
  "msg": "Get bch account info success",
  "network": "",
  "balance": "222.8"
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
  "chain": "BitcoinCash",
  "coin": "BCH",
  "network": "mainnet",
  "hash": "6ccd96206224634d495ddd634cd53e0fdbc8b2a64e91c76ce8bebee4ba40183a"
}' 127.0.0.1:8289 dapplink.utxo.WalletUtxoService.getTxByHash
```

- response
```
{
  "code": "SUCCESS",
  "msg": "get transaction detail success",
  "tx": {
    "hash": "6ccd96206224634d495ddd634cd53e0fdbc8b2a64e91c76ce8bebee4ba40183a",
    "index": 0,
    "froms": [
      {
        "address": "pz27rfvvsay2aqjp2g62dlzwqkfas56wzgcvuwd4ae"
      },
      {
        "address": "pquptsgc3ce7uz07hycdy4ct9uzv925xy5qr4t59ju"
      },
      {
        "address": "prwh6n40lcgfpv47wcrepevn8k240vdt7sc8u2uyxy"
      },
      {
        "address": "pzgzy9ctyt5laj2gjpgyl2r7mhynu4k4zg52kljq5q"
      },
      {
        "address": "pzcy2k76cx2at6tm2fx4l98djh5snvjsq5ure9x5lr"
      },
      {
        "address": "prm8zzv4lpk7dqxscj6pd0uqhpylc4js7vuhsdurlu"
      },
      {
        "address": "pq6a3mgkjkrcldcjx5xhuc9skjuwgz9jpu4qck5zxf"
      },
      {
        "address": "prwh6n40lcgfpv47wcrepevn8k240vdt7sc8u2uyxy"
      },
      {
        "address": "prz9759zzsaeya9x5ffprqew7aked7x5kgxywzw7m9"
      },
      {
        "address": "prwh6n40lcgfpv47wcrepevn8k240vdt7sc8u2uyxy"
      }
    ],
    "tos": [
      {
        "address": "pz27rfvvsay2aqjp2g62dlzwqkfas56wzgcvuwd4ae"
      },
      {
        "address": "pquptsgc3ce7uz07hycdy4ct9uzv925xy5qr4t59ju"
      },
      {
        "address": "prwh6n40lcgfpv47wcrepevn8k240vdt7sc8u2uyxy"
      },
      {
        "address": "pzgzy9ctyt5laj2gjpgyl2r7mhynu4k4zg52kljq5q"
      },
      {
        "address": "pzcy2k76cx2at6tm2fx4l98djh5snvjsq5ure9x5lr"
      },
      {
        "address": "prm8zzv4lpk7dqxscj6pd0uqhpylc4js7vuhsdurlu"
      },
      {
        "address": "pq6a3mgkjkrcldcjx5xhuc9skjuwgz9jpu4qck5zxf"
      },
      {
        "address": "prwh6n40lcgfpv47wcrepevn8k240vdt7sc8u2uyxy"
      },
      {
        "address": "prz9759zzsaeya9x5ffprqew7aked7x5kgxywzw7m9"
      },
      {
        "address": "prwh6n40lcgfpv47wcrepevn8k240vdt7sc8u2uyxy"
      },
      {
        "address": "pzt7dagm7w93d5hlewpv9g6575trqde7yvllw2yvpv"
      }
    ],
    "fee": "0.00006572",
    "status": "Success",
    "values": [
      {
        "value": "111.2"
      },
      {
        "value": "111.6"
      },
      {
        "value": "133.5"
      },
      {
        "value": "136"
      },
      {
        "value": "163.5"
      },
      {
        "value": "165.8"
      },
      {
        "value": "167.4"
      },
      {
        "value": "167.8"
      },
      {
        "value": "3195.75898437"
      }
    ],
    "type": 0,
    "height": "870667",
    "brc20_address": "",
    "datetime": "1730681749000"
  }
}
```

## 5.get address utxo
- request
```
grpcurl -plaintext -d '{
  "chain": "BitcoinCash",
  "network": "mainnet",
  "address": "pqwjhsy0qsj8e6drfm5gua6lcdlqagczjq7vh5kak6"
}' 127.0.0.1:8289 dapplink.utxo.WalletUtxoService.getUnspentOutputs
```
- response
```
{
  "code": "SUCCESS",
  "msg": "get bitcoin cash utxo success",
  "unspent_outputs": [
    {
      "tx_id": "6ccd96206224634d495ddd634cd53e0fdbc8b2a64e91c76ce8bebee4ba40183a",
      "tx_hash_big_endian": "",
      "tx_output_n": "1",
      "script": "",
      "height": "870667",
      "block_time": "1730681749",
      "address": "pqwjhsy0qsj8e6drfm5gua6lcdlqagczjq7vh5kak6",
      "unspent_amount": "111.6",
      "value_hex": "",
      "confirmations": "0",
      "index": "1"
    },
    {
      "tx_id": "6ccd96206224634d495ddd634cd53e0fdbc8b2a64e91c76ce8bebee4ba40183a",
      "tx_hash_big_endian": "",
      "tx_output_n": "0",
      "script": "",
      "height": "870667",
      "block_time": "1730681749",
      "address": "pqwjhsy0qsj8e6drfm5gua6lcdlqagczjq7vh5kak6",
      "unspent_amount": "111.2",
      "value_hex": "",
      "confirmations": "0",
      "index": "0"
    }
  ]
}
```