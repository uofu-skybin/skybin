package metaserver

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"skybin/core"
)

type jsonDB struct {
	providers []core.ProviderInfo
	renters   []core.RenterInfo
	files     []core.File
	path      string
}

type storageFile struct {
	Providers []core.ProviderInfo
	Renters   []core.RenterInfo
}

func newJsonDB(dbLocation string) jsonDB {
	db := jsonDB{}

	if _, err := os.Stat(dbLocation); os.IsNotExist(err) {
		db.dumpDbToFile()
	} else {
		db.loadDbFromFile()
	}

	return db
}

func (db jsonDB) dumpDbToFile() {
	storageFile := storageFile{Providers: db.providers, Renters: db.renters}

	dbBytes, err := json.Marshal(storageFile)
	if err != nil {
		panic(err)
	}

	writeErr := ioutil.WriteFile(path.Join(db.path), dbBytes, 0644)
	if writeErr != nil {
		panic(err)
	}
}

func (db jsonDB) loadDbFromFile() {
	contents, err := ioutil.ReadFile(path.Join(db.path))
	if err != nil {
		panic(err)
	}

	var newInfo storageFile

	parseErr := json.Unmarshal(contents, &newInfo)
	if parseErr != nil {
		panic(parseErr)
	}

	db.providers = newInfo.Providers
	db.renters = newInfo.Renters
}

// Renter operations

func (db jsonDB) FindAllRenters() []core.RenterInfo {
	return db.renters
}

func (db jsonDB) FindRenterByID(id string) (core.RenterInfo, error) {
	for _, renter := range db.renters {
		if renter.ID == id {
			return renter, nil
		}
	}
	return core.RenterInfo{}, errors.New("could not locate renter with given ID")
}

func (db jsonDB) InsertRenter(newRenter core.RenterInfo) error {
	for _, renter := range db.renters {
		if newRenter.ID == renter.ID {
			return errors.New("renter with given ID already exists")
		}
	}
	db.renters = append(db.renters, newRenter)
	db.dumpDbToFile()
	return nil
}

func (db jsonDB) UpdateRenter(updateRenter core.RenterInfo) error {
	foundRenter := false
	var replaceIndex int

	for i, renter := range db.renters {
		if updateRenter.ID == renter.ID {
			replaceIndex = i
			foundRenter = true
		}
	}

	if !foundRenter {
		return errors.New("could not find renter with given ID")
	}

	db.renters[replaceIndex] = updateRenter
	db.dumpDbToFile()
	return nil
}

func (db jsonDB) DeleteRenter(renterID string) error {
	removeIndex := -1
	for i, renter := range db.renters {
		if renterID == renter.ID {
			removeIndex = i
		}
	}
	if removeIndex == -1 {
		return errors.New("no renter with given ID")
	}
	db.renters = append(db.renters[:removeIndex], db.renters[removeIndex+1:]...)
	db.dumpDbToFile()
	return nil
}

// Provider operations
func (db jsonDB) FindAllProviders() []core.ProviderInfo {
	return db.providers
}

func (db jsonDB) FindProviderByID(id string) (core.ProviderInfo, error) {
	for _, provider := range db.providers {
		if provider.ID == id {
			return provider, nil
		}
	}
	return core.ProviderInfo{}, errors.New("could not locate provider with given ID")
}

func (db jsonDB) InsertProvider(newProvider core.ProviderInfo) error {
	for _, provider := range db.providers {
		if newProvider.ID == provider.ID {
			return errors.New("provider with given ID already exists")
		}
	}
	db.providers = append(db.providers, newProvider)
	db.dumpDbToFile()
	return nil
}

func (db jsonDB) UpdateProvider(updateProvider core.ProviderInfo) error {
	foundProvider := false
	var replaceIndex int

	for i, provider := range db.providers {
		if updateProvider.ID == provider.ID {
			replaceIndex = i
			foundProvider = true
		}
	}

	if !foundProvider {
		return errors.New("could not find provider with given ID")
	}

	db.providers[replaceIndex] = updateProvider
	db.dumpDbToFile()
	return nil
}

func (db jsonDB) DeleteProvider(providerID string) error {
	removeIndex := -1
	for i, provider := range db.providers {
		if providerID == provider.ID {
			removeIndex = i
		}
	}
	if removeIndex == -1 {
		return errors.New("no provider with given ID")
	}
	db.providers = append(db.providers[:removeIndex], db.providers[removeIndex+1:]...)
	db.dumpDbToFile()
	return nil
}

// File operations

func (db jsonDB) FindAllFiles() []core.File {
	return db.files
}

func (db jsonDB) FindFileByID(fileID string) (core.File, error) {
	var foundFiles []core.File
	for _, file := range db.files {
		if file.ID == fileID {
			foundFiles = append(foundFiles, file)
		}
	}
	if len(foundFiles) == 0 {
		return core.File{}, errors.New("could not locate file with given ID")
	}
	highestVersionIndex := -1
	highestVersionFound := -1
	for i, file := range foundFiles {
		if file.VersionNum > highestVersionFound {
			highestVersionIndex = i
			highestVersionFound = file.VersionNum
		}
	}
	return foundFiles[highestVersionIndex], nil
}

func (db jsonDB) FindFileByIDAndVersion(fileID string, version int) (core.File, error) {
	for _, file := range db.files {
		if file.ID == fileID && file.VersionNum == version {
			return file, nil
		}
	}
	return core.File{}, errors.New("no file with specified ID and version")
}

func (db jsonDB) InsertFile(newFile core.File) error {
	var renter core.RenterInfo
	foundRenter := false
	for _, item := range db.renters {
		if item.ID == newFile.OwnerID {
			foundRenter = true
		}
	}
	if !foundRenter {
		return errors.New("no renter matching ownerID")
	}
	for _, file := range db.files {
		if file.ID == newFile.ID && file.VersionNum == newFile.VersionNum {
			return errors.New("there is already a file with the given ID and version")
		}
	}
	db.files = append(db.files, newFile)
	db.dumpDbToFile()
	return nil
}

func (db jsonDB) UpdateFile(updateFile core.File) error {
	var renter core.RenterInfo
	foundRenter := false
	for _, item := range db.renters {
		if item.ID == updateFile.OwnerID {
			foundRenter = true
		}
	}
	if !foundRenter {
		return errors.New("no renter matching ownerID")
	}
	updateIndex := -1
	for i, file := range db.files {
		if file.ID == updateFile.ID && file.VersionNum == updateFile.VersionNum {
			updateIndex = i
		}
	}
	if updateIndex == -1 {
		return errors.New("could not locate file with specified ID and version")
	}
	db.files[updateIndex] = updateFile
	db.dumpDbToFile()
	return nil
}

func (db jsonDB) DeleteFile(fileID string) error {
	var keepFiles []core.File
	for _, file := range db.files {
		if file.ID != fileID {
			keepFiles = append(keepFiles, file)
		}
	}
	if len(keepFiles) == len(db.files) {
		return errors.New("no files with specified ID")
	}
	db.files = keepFiles
	db.dumpDbToFile()
	return nil
}

func (db jsonDB) DeleteFileVersion(fileID string, versionNum int) error {
	deleteIndex := -1
	for i, file := range db.files {
		if file.ID == fileID && file.VersionNum == versionNum {
			deleteIndex = i
		}
	}
	if deleteIndex == -1 {
		return errors.New("could not locate file with specified ID and version")
	}
	db.files = append(db.files[:deleteIndex], db.files[deleteIndex+1:]...)
	db.dumpDbToFile()
	return nil
}
