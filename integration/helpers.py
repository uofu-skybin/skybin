
from constants import *
import json
import requests

def reserve_space(amount):
    resp = requests.post(RENTER_ADDR + '/storage', json={'amount': amount})
    if resp.status_code != 201:
        raise ValueError(resp.content.decode('utf-8'))        

def upload_file(source, dest):
    resp = requests.post(RENTER_ADDR + '/files', json={
        'sourcePath': source,
        'destPath': dest
    })
    if resp.status_code != 201:
        raise ValueError(resp.content.decode('utf-8'))        
    return json.loads(resp.content)

def download_file(file_id, destination):
    url = '{}/files/{}/download'.format(RENTER_ADDR, file_id)
    resp = requests.post(url, json={'destination': destination})
    if resp.status_code != 201:
        raise ValueError(resp.content.decode('utf-8'))        
    
def share_file(file_id, user_id):
    resp = requests.post(RENTER_ADDR + '/files/{}/permissions'.format(file_id), json={
        'userId': user_id
    })
    if resp.status_code != 201:
        raise ValueError(resp.content.decode('utf-8'))
    return json.loads(resp.content)

def remove_file(file_id):
    url = '{}/files/{}'.format(RENTER_ADDR, file_id)
    resp = requests.delete(url)
    if resp.status_code != 200:
        raise ValueError(resp.content.decode('utf-8'))

def list_files():
    resp = requests.get(RENTER_ADDR + '/files')
    if resp.status_code != 200:
        raise ValueError(resp.content.decode('utf-8'))
    return json.loads(resp.content)
