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
	renters   []RenterInfo
	files     []File
	path      string
}

type storageFile struct {
	Providers []core.ProviderInfo
	Renters   []RenterInfo
	Files     []File
}

func newJsonDB(dbLocation string) jsonDB {
	db := jsonDB{
		path:      dbLocation,
		providers: make([]core.ProviderInfo, 0),
		renters:   make([]RenterInfo, 0),
	}

	if _, err := os.Stat(dbLocation); os.IsNotExist(err) {
		_, err := os.Create(dbLocation)
		if err != nil {
			panic(err)
		}
		db.dumpDbToFile()
	} else {
		db.loadDbFromFile()
	}

	return db
}

func (db *jsonDB) dumpDbToFile() {
	storageFile := storageFile{Providers: db.providers, Renters: db.renters}

	dbBytes, err := json.Marshal(storageFile)
	if err != nil {
		panic(err)
	}

	writeErr := ioutil.WriteFile(path.Join(db.path), dbBytes, 0644)
	if writeErr != nil {
		panic(writeErr)
	}
}

func (db *jsonDB) loadDbFromFile() {
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

func (db *jsonDB) FindAllRenters() ([]RenterInfo, error) {
	return db.renters, nil
}

func (db *jsonDB) FindRenterByID(id string) (*RenterInfo, error) {
	for _, renter := range db.renters {
		if renter.ID == id {
			return &renter, nil
		}
	}
	return nil, errors.New("could not locate renter with given ID")
}

func (db *jsonDB) InsertRenter(newRenter RenterInfo) error {
	for _, renter := range db.renters {
		if newRenter.ID == renter.ID {
			return errors.New("renter with given ID already exists")
		}
	}
	db.renters = append(db.renters, newRenter)
	db.dumpDbToFile()
	return nil
}

func (db *jsonDB) UpdateRenter(updateRenter RenterInfo) error {
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

func (db *jsonDB) DeleteRenter(renterID string) error {
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
func (db *jsonDB) FindAllProviders() ([]core.ProviderInfo, error) {
	return db.providers, nil
}

func (db *jsonDB) FindProviderByID(id string) (*core.ProviderInfo, error) {
	for _, provider := range db.providers {
		if provider.ID == id {
			return &provider, nil
		}
	}
	return nil, errors.New("could not locate provider with given ID")
}

func (db *jsonDB) InsertProvider(newProvider core.ProviderInfo) error {
	for _, provider := range db.providers {
		if newProvider.ID == provider.ID {
			return errors.New("provider with given ID already exists")
		}
	}
	db.providers = append(db.providers, newProvider)
	db.dumpDbToFile()
	return nil
}

func (db *jsonDB) UpdateProvider(updateProvider core.ProviderInfo) error {
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

func (db *jsonDB) DeleteProvider(providerID string) error {
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

func (db *jsonDB) FindAllFiles() ([]File, error) {
	return db.files, nil
}

func (db *jsonDB) FindFileByID(fileID string) (*File, error) {
	for _, file := range db.files {
		if file.ID == fileID {
			return &file, nil
		}
	}
	return nil, errors.New("could not find file with specified ID")
}

func (db *jsonDB) FindFilesByRenter(renterID string) ([]File, error) {
	var renterFiles []File
	for _, item := range db.files {
		if item.OwnerID == renterID {
			renterFiles = append(renterFiles, item)
		}
	}
	return renterFiles, nil
}

func (db *jsonDB) InsertFile(newFile File) error {
	foundRenter := false
	for _, item := range db.renters {
		if item.ID == newFile.OwnerID {
			foundRenter = true
		}
	}
	if !foundRenter {
		return errors.New("no renter matching ownerID")
	}
	db.files = append(db.files, newFile)
	db.dumpDbToFile()
	return nil
}

func (db *jsonDB) UpdateFile(updateFile File) error {
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
		if file.ID == updateFile.ID {
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

func (db *jsonDB) DeleteFile(fileID string) error {
	removeIndex := -1
	for i, file := range db.files {
		if fileID == file.ID {
			removeIndex = i
		}
	}
	if removeIndex == -1 {
		return errors.New("no file with given ID")
	}
	db.files = append(db.files[:removeIndex], db.files[removeIndex+1:]...)
	db.dumpDbToFile()
	return nil
}
