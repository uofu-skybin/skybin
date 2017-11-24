
from constants import *
import requests

def reserve_space(amount):
    resp = requests.post(RENTER_ADDR + '/storage', json={'amount': amount})
    if resp.status_code != 201:
        raise ValueError('error: {}'.format(resp.content.decode('utf-8')))
