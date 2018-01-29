
import json
import requests

class RenterAPI:
    """Renter HTTP API wrapper"""

    def __init__(self, base_url):
        self.base_url = base_url

    def reserve_space(self, amount):
        resp = requests.post(self.base_url + '/reserve-storage', json={'amount': amount})
        if resp.status_code != 201:
            raise ValueError(resp.content.decode('utf-8'))

    def upload_file(self, source, dest):
        resp = requests.post(self.base_url + '/files/upload', json={
            'sourcePath': source,
            'destPath': dest
        })
        if resp.status_code != 201:
            raise ValueError(resp.content.decode('utf-8'))
        return json.loads(resp.content)

    def download_file(self, file_id, destination):
        url = self.base_url + '/files/download'
        resp = requests.post(url, json={
            'fileId': file_id,
            'destPath': destination,
        })
        if resp.status_code != 201:
            raise ValueError(resp.content.decode('utf-8'))

    def share_file(self, file_id, user_id):
        resp = requests.post(self.base_url + '/files/{}/permissions'.format(file_id), json={
            'userId': user_id
        })
        if resp.status_code != 201:
            raise ValueError(resp.content.decode('utf-8'))
        return json.loads(resp.content)

    def remove_file(self, file_id):
        url = '{}/files/{}'.format(self.base_url, file_id)
        resp = requests.delete(url)
        if resp.status_code != 200:
            raise ValueError(resp.content.decode('utf-8'))

    def list_files(self):
        resp = requests.get(self.base_url + '/files')
        if resp.status_code != 200:
            raise ValueError(resp.content.decode('utf-8'))
        return json.loads(resp.content)
