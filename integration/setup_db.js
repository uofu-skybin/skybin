const dbAddr = '127.0.0.1/skybin'
var db = connect(dbAddr)

db.renters.createIndex({"alias": 1, "ID": 1}, {unique: true})
db.providers.createIndex({"ID": 1}, {unique: true})
db.files.createIndex({"ID": 1}, {unique: true})
db.files.createIndex({"name": 1, "ownerID": 1}, {unique: true})