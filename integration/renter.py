
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
        return json.loads(resp.content.decode('utf-8'))

    def reserve_space(self, amount):
        resp = requests.post(self.base_url + '/reserve-storage', json={'amount': amount})
        if resp.status_code != 201:
            raise ValueError(resp.content.decode('utf-8'))

    def upload_file(self, source, dest, should_overwrite=None):
        args = {
            'sourcePath': source,
            'destPath': dest,
        }
        if should_overwrite != None:
            args['shouldOverwrite'] = should_overwrite
        resp = requests.post(self.base_url + '/files/upload', json=args)
        if resp.status_code != 201:
            raise ValueError(resp.content.decode('utf-8'))
        return json.loads(resp.content.decode('utf-8'))

    def download_file(self, file_id, destination, version_num=None):
        url = self.base_url + '/files/download'
        args = {
            'fileId': file_id,
            'destPath': destination,
        }
        if version_num != None:
            args['versionNum'] = version_num
        resp = requests.post(url, json=args)
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
        return json.loads(resp.content.decode('utf-8'))

    def create_folder(self, name):
        url = self.base_url + '/files/create-folder'
        resp = requests.post(url, json={
            'name': name,
        })
        if resp.status_code != 201:
            raise ValueError(resp.content.decode('utf-8'))
        return json.loads(resp.content.decode('utf-8'))

    def share_file(self, file_id, renter_alias):
        resp = requests.post(self.base_url + '/files/share', json={
            'fileId': file_id,
            'renterAlias': renter_alias
        })
        if resp.status_code != 200:
            raise ValueError(str(resp.status_code) + ' ' + resp.content.decode('utf-8'))
        return json.loads(resp.content.decode('utf-8'))

    def remove_file(self, file_id, version_num=None):
        url = '{}/files/remove'.format(self.base_url)
        args = {'fileID': file_id}
        if version_num != None:
            args['versionNum'] = version_num
        resp = requests.post(url, json=args)
        if resp.status_code != 200:
            raise ValueError(resp.content.decode('utf-8'))

    def list_files(self):
        resp = requests.get(self.base_url + '/files')
        if resp.status_code != 200:
            raise ValueError(resp.content.decode('utf-8'))
        return json.loads(resp.content.decode('utf-8'))

    def list_shared_files(self):
        resp = requests.get(self.base_url + '/files/shared')
        if resp.status_code != 200:
            raise ValueError(resp.content.decode('utf-8'))
        return json.loads(resp.content.decode('utf-8'))
