# docdb

A simple Go document database. [blog: Writing a document database from scratch in Go: Lucene-like filters and indexes](https://notes.eatonphil.com/documentdb.html)

## Usage

1. `make install`
2. Startup: `docdb`
3. download [movie.json](https://github.com/prust/wikipedia-movie-data) by [gurl](https://github.com/bingoohuang/gurl): `gurl https://github.com/prust/wikipedia-movie-data/raw/master/movies.json`
4. load movie.json into docdb: `sh scripts/load_array.sh movies.json` 
   1. 28795 (`jj -i movies.json #` or `jq length movies.json`) movie json, took 39m51s on my laptop.
   2. `jq -c '.[]' movies.json | gurl :8080/docs -n0 -pbv -r`, took 34m47s on my laptop.
   3. or `jj -I -i movies.json | gurl :8080/docs -n0 -r -pb`
   4. Asynchronous version, took 7s on my laptop.
5. query: `gurl :8080/docs 'q==title:"New Life Rescue"'`

## pebble vs lotusdb vs pogreb

28795 条 JSON 导入: `time (jj -I -i movies.json | gurl :8080/docs -n0 -r -pb)`

1. [pebble](https://github.com/cockroachdb/pebble)  `3.29s user 2.19s system 69% cpu 7.861 total`
2. [lotusdb](https://github.com/flower-corp/lotusdb) `3.38s user 2.30s system 88% cpu 6.426 total`
2. [pogreb](https://github.com/akrylysov/pogreb) `3.01s user 1.96s system 82% cpu 6.048 total`

## scripts

Then in another terminal:

```bash
$ curl -X POST -H 'Content-Type: application/json' -d '{"name": "Kevin", "age": "45"}' http://localhost:8080/docs
{"body":{"id":"5ac64e74-58f9-4ba4-909e-1d5bf4ddcaa1"},"status":"ok"}
$ curl --get http://localhost:8080/docs --data-urlencode 'q=name:"Kevin"' | jq
{
  "body": {
    "count": 1,
    "documents": [
      {
        "body": {
          "age": "45",
          "name": "Kevin"
        },
        "id": "5ac64e74-58f9-4ba4-909e-1d5bf4ddcaa1"
      }
    ]
  },
  "status": "ok"
}
$ curl --get http://localhost:8080/docs --data-urlencode 'q=age:<50' | jq
{
  "body": {
    "count": 1,
    "documents": [
      {
        "body": {
          "age": "45",
          "name": "Kevin"
        },
        "id": "5ac64e74-58f9-4ba4-909e-1d5bf4ddcaa1"
      }
    ]
  },
  "status": "ok"
}
```

## gurl

```sh
🕙[2022-04-02 22:19:12.360] ❯ gurl POST :8080/docs name=Kevin age:=45 -pb
{
  "body": {
    "id": "27FHzv8h0T8gx1Qcy9ZR8qGMI4c"
  },
  "status": "ok"
}

🕙[2022-04-02 22:19:39.922] ❯ gurl :8080/docs q==name:Kevin -pb
{
  "body": {
    "count": 1,
    "documents": [
      {
        "body": {
          "age": 45,
          "name": "Kevin"
        },
        "id": "27FHzv8h0T8gx1Qcy9ZR8qGMI4c"
      }
    ]
  },
  "status": "ok"
}

🕙[2022-04-02 22:20:36.344] ❯ gurl :8080/docs 'q==age:<50' -pb
{
  "body": {
    "count": 1,
    "documents": [
      {
        "body": {
          "age": 45,
          "name": "Kevin"
        },
        "id": "27FHzv8h0T8gx1Qcy9ZR8qGMI4c"
      }
    ]
  },
  "status": "ok"
}

🕙[2022-04-02 22:21:12.931] ❯ gurl :8080/docs q=='age:<40' -pb
{
  "body": {
    "count": 0,
    "documents": null
  },
  "status": "ok"
}
```
