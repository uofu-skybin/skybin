
"""
Test for creating and listing contracts.
"""

import argparse
import filecmp
from test_framework import setup_test

def contracts_test(ctxt):
    contract_ids = [c['contractId'] for c in ctxt.renter.reserve_space(int(5e9))]
    contract_ids2 = [c['contractId'] for c in ctxt.renter.list_contracts()]
    ctxt.assert_true(set(contract_ids) == set(contract_ids2))

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=1,
                        help='number of providers to run')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
    )
    try:
        ctxt.log('contracts test')
        contracts_test(ctxt)
        ctxt.log('ok')
    finally:
        ctxt.teardown()

if __name__ == "__main__":
    main()
