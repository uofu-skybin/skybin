const dbAddr = '127.0.0.1/skybin'
var db = connect(dbAddr)

db.renters.createIndex({"id": 1}, {unique: true})
db.renters.createIndex({"alias": 1}, {unique: true})

db.providers.createIndex({"id": 1}, {unique: true})

db.files.createIndex({"id": 1}, {unique: true})
db.files.createIndex({"name": 1, "ownerid": 1}, {unique: true})