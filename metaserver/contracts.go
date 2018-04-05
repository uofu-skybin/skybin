package metaserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"skybin/core"
	"skybin/util"
	"time"

	"github.com/gorilla/mux"
)

type contractResp struct {
	Contract core.Contract `json:"file,omitempty"`
	Error    string        `json:"error,omitempty"`
}

func (server *MetaServer) getContractsHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		// Make sure the person making the request is the renter who owns the files.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != params["renterID"] {
			writeErr("cannot access other users' contracts", http.StatusUnauthorized, w)
			return
		}

		// Make sure the specified renter actually exists.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		// Retrieve the renter's contracts.
		contracts, err := server.db.FindContractsByRenter(renter.ID)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		json.NewEncoder(w).Encode(contracts)
	})
}

func (server *MetaServer) postContractHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var contract core.Contract
		err := json.NewDecoder(r.Body).Decode(&contract)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		params := mux.Vars(r)

		// Make sure the person making the request is the renter who owns the files.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		// Make sure the renter exists.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		if renterID, present := claims["renterID"]; !present || renterID.(string) != contract.RenterId {
			writeErr("cannot post contracts for other users", http.StatusUnauthorized, w)
			return
		}

		// TEST CODE! Make every contract cost 100 bucks for testing.
		// contract.Fee = 100 * 1000
		// MORE TEST CODE! Make the duration of the contract 1 week.
		// contract.EndDate = time.Now().Add(time.Hour * 24 * 7)

		// Make sure the renter has enough money to pay for the contract.
		if contract.Fee > renter.Balance {
			writeErr("cannot afford contract", http.StatusBadRequest, w)
			return
		}

		// Subtract the balance of the contract from the renter's account.
		// TODO: Add atomic DB operations to increment and decrement renter balances.
		renter.Balance -= contract.Fee
		err = server.db.UpdateRenter(renter)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		startTime := time.Now()

		// Create a payment for the contract and insert it into the database.
		payment := &core.PaymentInfo{
			Contract:        contract.ID,
			Balance:         contract.Fee,
			LastPaymentTime: startTime,
			IsPaying:        true,
		}
		err = server.db.InsertPayment(payment)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		contract.StartDate = startTime
		err = server.db.InsertContract(&contract)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Create a transaction showing the contract.
		transaction := &core.Transaction{
			UserType:        "renter",
			UserID:          renter.ID,
			ContractID:      contract.ID,
			TransactionType: "payment",
			Amount:          contract.Fee,
			Date:            startTime,
			Description:     fmt.Sprintf("Contract %s formed with %s", contract.ID, contract.ProviderId),
		}
		err = server.db.InsertTransaction(transaction)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(contract)
	})
}

func (server *MetaServer) getContractHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		contract, err := server.db.FindContractByID(params["contractID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}

		// Make sure the person making the request is the renter who owns the files.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != contract.RenterId {
			writeErr("cannot retrieve other users' contracts", http.StatusUnauthorized, w)
			return
		}

		json.NewEncoder(w).Encode(contract)
	})
}

func (server *MetaServer) putContractHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		var contract core.Contract
		err := json.NewDecoder(r.Body).Decode(&contract)
		if err != nil {
			writeErr("could not parse payload", http.StatusBadRequest, w)
			return
		}

		if contract.ID != params["contractID"] {
			writeErr("must not change contact ID", http.StatusUnauthorized, w)
			return
		}

		oldContract, err := server.db.FindContractByID(contract.ID)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Make sure the person making the request is the renter who owns the contract.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != oldContract.RenterId {
			writeErr("cannot modify other users' contracts", http.StatusUnauthorized, w)
			return
		}

		err = server.db.UpdateContract(&contract)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (server *MetaServer) deleteContractHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		contract, err := server.db.FindContractByID(params["contractID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Make sure the person making the request is the renter who owns the contract.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != contract.RenterId {
			writeErr("cannot delete other users' contracts", http.StatusUnauthorized, w)
			return
		}

		err = server.db.DeleteContract(params["contractID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (server *MetaServer) getContractPaymentHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		contract, err := server.db.FindContractByID(params["contractID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}

		// Make sure the person making the request is the renter who owns the files.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != contract.RenterId {
			writeErr("cannot retrieve other users' contracts", http.StatusUnauthorized, w)
			return
		}

		// Retrieve the contract's payment information.
		payment, err := server.db.FindPaymentByContract(contract.ID)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		json.NewEncoder(w).Encode(payment)
	})
}

func (server *MetaServer) putContractPaymentHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		var payload core.PaymentInfo
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			writeErr("Could not decode payload", http.StatusBadRequest, w)
			return
		}

		contract, err := server.db.FindContractByID(params["contractID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}

		// Make sure the person making the request is the renter who owns the files.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != contract.RenterId {
			writeErr("cannot retrieve other users' contracts", http.StatusUnauthorized, w)
			return
		}

		// Retrieve the contract's payment information.
		payment, err := server.db.FindPaymentByContract(contract.ID)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		// Make sure the only field the user is modifying is the "isPaying" field.
		if payload.Balance != payment.Balance ||
			payload.Contract != payment.Contract ||
			payload.LastPaymentTime != payment.LastPaymentTime {
			writeErr("Can only modify isPaying field", http.StatusBadRequest, w)
			return
		}

		// Update the payment.
		err = server.db.UpdatePayment(&payload)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
