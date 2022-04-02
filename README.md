# docdb

A simple Go document database. [blog: Writing a document database from scratch in Go: Lucene-like filters and indexes](https://notes.eatonphil.com/documentdb.html)

## Usage

1. `make install`
2. Startup: `docdb`
3. download [movie.json](https://github.com/prust/wikipedia-movie-data): `gurl https://github.com/prust/wikipedia-movie-data/raw/master/movies.json -d`
4. load movie.json into docdb: `sh scripts/load_array.sh movies.json`

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
