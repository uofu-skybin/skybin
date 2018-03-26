package provider

import (
	"database/sql"
	"fmt"
	"skybin/core"
	"time"
)

func (p *Provider) setup_db(path string) (*sql.DB, error) {
	db, _ := sql.Open("sqlite3", path)
	// Create contracts table
	stmt, err := db.Prepare("CREATE TABLE IF NOT EXISTS contracts (id INTEGER PRIMARY KEY, ContractId TEXT, RenterId TEXT, ProviderId TEXT, StorageSpace INTEGER, StartDate TEXT, EndDate TEXT, RenterSignature TEXT, ProviderSignature TEXT)")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	stmt.Exec()

	// Create blocks table
	stmt, err = db.Prepare("CREATE TABLE IF NOT EXISTS blocks (id INTEGER PRIMARY KEY, RenterId TEXT, BlockId TEXT, Size INTEGER)")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	stmt.Exec()

	// Create renters table
	stmt, err = db.Prepare("CREATE TABLE IF NOT EXISTS renters (id INTEGER PRIMARY KEY, RenterId TEXT, StorageReserved INTEGER, StorageUsed INTEGER)")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	stmt.Exec()

	return db, err
}

func (p *Provider) InsertBlock(renterId string, blockId string, size int64) error {
	stmt, err := p.db.Prepare("INSERT INTO blocks (RenterId, BlockId, Size) VALUES (?, ?, ?)")
	if err != nil {
		fmt.Println(err)
		return err
	}
	_, err = stmt.Exec(renterId, blockId, size)
	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func (p *Provider) DeleteBlockById(blockId string) error {
	stmt, err := p.db.Prepare("DELETE from blocks where BlockId=?")
	if err != nil {
		fmt.Println(err)
		return err
	}
	_, err = stmt.Exec(blockId)
	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func (p *Provider) InsertContract(contract *core.Contract) error {
	stmt, err := p.db.Prepare("INSERT INTO contracts (ContractId, RenterId, ProviderId, StorageSpace, StartDate, EndDate, RenterSignature, ProviderSignature) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		fmt.Println(err)
		return err
	}
	_, err = stmt.Exec(contract.ID,
		contract.RenterId,
		contract.ProviderId,
		contract.StorageSpace,
		contract.StartDate.Format(time.RFC3339),
		contract.EndDate.Format(time.RFC3339),
		contract.RenterSignature,
		contract.ProviderSignature,
	)
	return nil
}

func (p *Provider) GetContractsByRenter(renterId string) ([]*core.Contract, error) {
	query := fmt.Sprintf("SELECT ContractId, RenterId, ProviderId, StorageSpace, RenterSignature, ProviderSignature, StartDate, EndDate FROM contracts where RenterId='%s'", renterId)
	rows, err := p.db.Query(query)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	var contracts []*core.Contract

	for rows.Next() {
		c := &core.Contract{}
		// scan does not parse these directly into time.Time
		var startDate string
		var endDate string
		rows.Scan(&c.ID, &c.RenterId, &c.ProviderId, &c.StorageSpace, &c.RenterSignature, &c.ProviderSignature, &startDate, &endDate)
		c.StartDate, _ = time.Parse(time.RFC3339, startDate)
		c.EndDate, _ = time.Parse(time.RFC3339, endDate)
		contracts = append(contracts, c)
	}
	return contracts, nil
}
