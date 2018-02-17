
"""
Tests for the file rename operation.
"""

from test_framework import setup_test

def rename_test(ctxt):
    ctxt.log('rename test')

    ctxt.renter.reserve_space(1 * 1024 * 1024 * 1024)
    input_file = ctxt.create_test_file(size=1024*1024)

    # Renaming a file should succeed
    f1 = ctxt.renter.upload_file(input_file, 'file1')
    f1 = ctxt.renter.rename_file(f1['id'], 'file1_new')
    ctxt.assert_true(f1['name'] == 'file1_new')

    # Renaming a folder should succeed
    folder = ctxt.renter.create_folder('folder')
    folder = ctxt.renter.rename_file(folder['id'], 'folder1')
    ctxt.assert_true(folder['name'] == 'folder1')

    # Renaming a folder should rename all of its children
    folder2 = ctxt.renter.create_folder('folder2')
    ctxt.renter.create_folder('folder2/subfolder')
    ctxt.renter.create_folder('folder2/subfolder/subfolder')
    ctxt.renter.upload_file(input_file, 'folder2/file')
    ctxt.renter.rename_file(folder2['id'], 'folder2_new')
    file_names = [f['name'] for f in ctxt.renter.list_files()['files']]
    ctxt.assert_true('folder2_new' in file_names)
    ctxt.assert_true('folder2_new/subfolder' in file_names)
    ctxt.assert_true('folder2_new/subfolder/subfolder' in file_names)
    ctxt.assert_true('folder2_new/file' in file_names)

    # Renaming a folder should not rename non-children with the same prefix
    folder3 = ctxt.renter.create_folder('folder3')
    folder4 = ctxt.renter.create_folder('folder34')
    ctxt.renter.rename_file(folder3['id'], 'folder5')
    file_names = [f['name'] for f in ctxt.renter.list_files()['files']]
    ctxt.assert_true('folder34' in file_names)

    # Renaming an invalid file ID should fail
    try:
        ctxt.renter.rename_file('not a file ID', 'some name')
        ctxt.assert_true(False, 'Renamed non-existent file')
    except Exception:
        pass    

    # Renaming a file to an already-used name should fail
    f2 = ctxt.renter.upload_file(input_file, 'file2')
    try:
        ctxt.renter.rename_file(f1['id'], f2['name'])
        ctxt.assert_true(False, 'Renamed file ')
    except Exception:
        pass

    ctxt.log('ok')

def main():
    ctxt = setup_test()
    try:
        rename_test(ctxt)
    finally:
        ctxt.teardown()

if __name__ == '__main__':
    main()
