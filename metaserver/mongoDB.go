package metaserver

import (
	"skybin/core"

	"github.com/globalsign/mgo"
)

const dbName = "skybin"
const dbAddress = "127.0.0.1"

type mongoDB struct {
	session *mgo.Session
}

func newMongoDB() (*mongoDB, error) {
	session, err := mgo.Dial(dbAddress)
	if err != nil {
		return nil, err
	}

	db := mongoDB{session: session}
	return &db, nil
}

func (db *mongoDB) CloseDB() {
	db.session.Close()
}

func (db *mongoDB) getMongoCollection(name string) (*mgo.Collection, *mgo.Session, error) {
	session := db.session.Copy()
	c := session.DB(dbName).C(name)

	return c, session, nil
}

// Renter operations
//==================

// Return a list of all renters in the database
func (db *mongoDB) FindAllRenters() ([]core.RenterInfo, error) {
	c, session, err := db.getMongoCollection("renters")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var result []core.RenterInfo
	err = c.Find(nil).All(&result)

	return result, nil
}

// Return the renter with the specified ID.
func (db *mongoDB) FindRenterByID(renterID string) (*core.RenterInfo, error) {
	c, session, err := db.getMongoCollection("renters")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	selector := struct{ ID string }{ID: renterID}
	var result core.RenterInfo
	err = c.Find(selector).One(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// Insert the provided renter into the database.
func (db *mongoDB) InsertRenter(renter *core.RenterInfo) error {
	c, session, err := db.getMongoCollection("renters")
	if err != nil {
		return err
	}
	defer session.Close()

	err = c.Insert(renter)
	if err != nil {
		return err
	}

	return nil
}

// Update the provided renter in the databse.
func (db *mongoDB) UpdateRenter(renter *core.RenterInfo) error {
	c, session, err := db.getMongoCollection("renters")
	if err != nil {
		return err
	}
	defer session.Close()

	selector := struct{ ID string }{ID: renter.ID}
	err = c.Update(selector, renter)
	if err != nil {
		return err
	}

	return nil
}

// Delete the specified renter from the database.
func (db *mongoDB) DeleteRenter(renterID string) error {
	c, session, err := db.getMongoCollection("renters")
	if err != nil {
		return err
	}
	defer session.Close()

	selector := struct{ ID string }{ID: renterID}
	err = c.Remove(selector)
	if err != nil {
		return err
	}

	return nil
}

// Provider operations
//====================

// Return a list of all the providers in the database.
func (db *mongoDB) FindAllProviders() ([]core.ProviderInfo, error) {
	c, session, err := db.getMongoCollection("providers")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var result []core.ProviderInfo
	err = c.Find(nil).All(&result)

	return result, nil
}

// Return the provider with the specified ID.
func (db *mongoDB) FindProviderByID(providerID string) (*core.ProviderInfo, error) {
	c, session, err := db.getMongoCollection("providers")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	selector := struct{ ID string }{ID: providerID}
	var result core.ProviderInfo
	err = c.Find(selector).One(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// Insert the given provider into the database.
func (db *mongoDB) InsertProvider(provider *core.ProviderInfo) error {
	c, session, err := db.getMongoCollection("providers")
	if err != nil {
		return err
	}
	defer session.Close()

	err = c.Insert(provider)
	if err != nil {
		return err
	}

	return nil
}

// Update the given provider in the databse.
func (db *mongoDB) UpdateProvider(provider *core.ProviderInfo) error {
	c, session, err := db.getMongoCollection("providers")
	if err != nil {
		return err
	}
	defer session.Close()

	selector := struct{ ID string }{ID: provider.ID}
	err = c.Update(selector, provider)
	if err != nil {
		return err
	}

	return nil
}

// Delete the specified provider from the dtabase.
func (db *mongoDB) DeleteProvider(providerID string) error {
	c, session, err := db.getMongoCollection("providers")
	if err != nil {
		return err
	}
	defer session.Close()

	selector := struct{ ID string }{ID: providerID}
	err = c.Remove(selector)
	if err != nil {
		return err
	}

	return nil
}

// File operations
//====================

// Return a list of all files in the database.
func (db *mongoDB) FindAllFiles() ([]core.File, error) {
	c, session, err := db.getMongoCollection("files")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var result []core.File
	err = c.Find(nil).All(&result)

	return result, nil
}

// Return the latest version of the file with the specified ID.
func (db *mongoDB) FindFileByID(fileID string) (*core.File, error) {
	c, session, err := db.getMongoCollection("files")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	selector := struct{ ID string }{ID: fileID}
	var result core.File
	err = c.Find(selector).One(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// Return a map of paths to files present in the renter's directory.
func (db *mongoDB) FindFilesInRenterDirectory(renterID string) ([]core.File, error) {
	session := db.session.Copy()
	defer session.Close()

	// Get file IDs in renter directory.
	renters := session.DB(dbName).C("renters")

	selector := struct{ ID string }{ID: renterID}
	var renter core.RenterInfo
	err := renters.Find(selector).One(&renter)
	if err != nil {
		return nil, err
	}
	filesToFind := renter.Files

	// Retrieve files from collection.
	files := session.DB(dbName).C("files")

	var foundFiles []core.File
	for _, item := range filesToFind {
		selector = struct{ ID string }{ID: item}
		var result core.File
		err = files.Find(selector).One(&result)
		if err != nil {
			return nil, err
		}
		foundFiles = append(foundFiles, result)
	}

	return foundFiles, nil
}

// Return a map of names to files shared with a given renter.
func (db *mongoDB) FindFilesSharedWithRenter(renterID string) ([]core.File, error) {
	session := db.session.Copy()
	defer session.Close()

	// Get file IDs in renter directory.
	renters := session.DB(dbName).C("renters")

	selector := struct{ ID string }{ID: renterID}
	var renter core.RenterInfo
	err := renters.Find(selector).One(&renter)
	if err != nil {
		return nil, err
	}
	filesToFind := renter.Shared

	// Retrieve files from collection.
	files := session.DB(dbName).C("files")

	var foundFiles []core.File
	for _, item := range filesToFind {
		selector = struct{ ID string }{ID: item}
		var result core.File
		err = files.Find(selector).One(&result)
		if err != nil {
			return nil, err
		}
		foundFiles = append(foundFiles, result)
	}

	return foundFiles, nil
}

// Return a list of files that the renter owns.
func (db *mongoDB) FindFilesByOwner(renterID string) ([]core.File, error) {
	c, session, err := db.getMongoCollection("files")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var result []core.File
	selector := struct{ ownerID string }{ownerID: renterID}
	err = c.Find(selector).All(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Insert the given file into the database.
func (db *mongoDB) InsertFile(file *core.File) error {
	c, session, err := db.getMongoCollection("files")
	if err != nil {
		return err
	}
	defer session.Close()

	err = c.Insert(file)
	if err != nil {
		return err
	}

	return nil
}

// Update the given file in the database.
func (db *mongoDB) UpdateFile(file *core.File) error {
	c, session, err := db.getMongoCollection("files")
	if err != nil {
		return err
	}
	defer session.Close()

	selector := struct{ ID string }{ID: file.ID}
	err = c.Update(selector, file)
	if err != nil {
		return err
	}

	return nil
}

// Delete all versions of the given file from the database.
func (db *mongoDB) DeleteFile(fileID string) error {
	c, session, err := db.getMongoCollection("files")
	if err != nil {
		return err
	}
	defer session.Close()

	selector := struct{ ID string }{ID: fileID}
	err = c.Remove(selector)
	if err != nil {
		return err
	}

	return nil
}

// Contract operations
//====================

// Return a list of all contracts in the database.
func (db *mongoDB) FindAllContracts() ([]core.Contract, error) {
	c, session, err := db.getMongoCollection("contracts")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var result []core.Contract
	err = c.Find(nil).All(&result)

	return result, nil
}

// Return the contract with the specified ID
func (db *mongoDB) FindContractByID(contractID string) (*core.Contract, error) {
	c, session, err := db.getMongoCollection("contracts")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	selector := struct{ ID string }{ID: contractID}
	var result core.Contract
	err = c.Find(selector).One(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// Return a list of contracts belonging to the specified renter.
func (db *mongoDB) FindContractsByRenter(renterID string) ([]core.Contract, error) {
	c, session, err := db.getMongoCollection("contracts")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var result []core.Contract
	selector := struct{ renterId string }{renterId: renterID}
	err = c.Find(selector).All(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Insert the given contract into the database.
func (db *mongoDB) InsertContract(contract *core.Contract) error {
	c, session, err := db.getMongoCollection("contracts")
	if err != nil {
		return err
	}
	defer session.Close()

	err = c.Insert(contract)
	if err != nil {
		return err
	}

	return nil
}

// Update the given contract.
func (db *mongoDB) UpdateContract(contract *core.Contract) error {
	c, session, err := db.getMongoCollection("contracts")
	if err != nil {
		return err
	}
	defer session.Close()

	selector := struct{ ID string }{ID: contract.ID}
	err = c.Update(selector, contract)
	if err != nil {
		return err
	}

	return nil
}

// Delete the contract.
func (db *mongoDB) DeleteContract(contractID string) error {
	c, session, err := db.getMongoCollection("contracts")
	if err != nil {
		return err
	}
	defer session.Close()

	selector := struct{ ID string }{ID: contractID}
	err = c.Remove(selector)
	if err != nil {
		return err
	}

	return nil
}