
import json
import requests

class RenterAPI:
    """Renter HTTP API wrapper"""

    def __init__(self, base_url):
        self.base_url = base_url

    def get_info(self):
        resp = requests.get(self.base_url + '/info')
        if resp.status_code != 200:
            raise ValueError(resp.content.decode('utf-8'))
        return json.loads(resp.content)

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

    def rename_file(self, file_id, name):
        url = self.base_url + '/files/rename'
        resp = requests.post(url, json={
            'fileId': file_id,
            'name': name,
        })
        if resp.status_code != 200:
            raise ValueError(resp.content.decode('utf-8'))
        return json.loads(resp.content)

    def create_folder(self, name):
        url = self.base_url + '/files/create-folder'
        resp = requests.post(url, json={
            'name': name,
        })
        if resp.status_code != 201:
            raise ValueError(resp.content.decode('utf-8'))
        return json.loads(resp.content)

    def share_file(self, file_id, user_id):
        resp = requests.post(self.base_url + '/files/share', json={
            'fileId': file_id,
            'renterId': user_id
        })
        if resp.status_code != 200:
            raise ValueError(str(resp.status_code) + ' ' + resp.content.decode('utf-8'))
        return json.loads(resp.content)

    def remove_file(self, file_id):
        url = '{}/files/remove'.format(self.base_url)
        resp = requests.post(url, json={'fileID': file_id})
        if resp.status_code != 200:
            raise ValueError(resp.content.decode('utf-8'))

    def list_files(self):
        resp = requests.get(self.base_url + '/files')
        if resp.status_code != 200:
            raise ValueError(resp.content.decode('utf-8'))
        return json.loads(resp.content)

    def list_shared_files(self):
        resp = requests.get(self.base_url + '/files/shared')
        if resp.status_code != 200:
            raise ValueError(resp.content.decode('utf-8'))
        return json.loads(resp.content)
