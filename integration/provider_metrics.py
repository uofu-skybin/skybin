import json
import random

with open("repo/provider/snapshot.json", "r+") as f:
    data = json.load(f)

rate = {}
rate["blockUploads"] = 20
rate["blockDownloads"] = 12
rate["blockDeletions"] = 8
rate["bytesUploaded"] = 150000000
rate["bytesDownloaded"] = 100000000
rate["storageReservations"] = 5

for i in range(0,24):
    for x in rate.keys():
        rand_blocks = random.randint(0,rate[x])
        data["stats"]["day"][x][i] = rand_blocks

for i in range(0,7):
    for x in rate.keys():
        rand_blocks = random.randint(0,rate[x])
        data["stats"]["week"][x][i] = rand_blocks * 7
        
with open("repo/provider/snapshot.json", 'w+') as f:
     json.dump(data, f)