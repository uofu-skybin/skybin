package metaserver

import (
	"encoding/json"
	"net/http"
	"skybin/util"

	"github.com/gorilla/mux"
)

func (server *MetaServer) getRenterTransactionsHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != params["renterID"] {
			writeErr("cannot access other users' transactions", http.StatusUnauthorized, w)
			return
		}

		// Make sure the specified renter actually exists.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		// Retrieve the renter's transactions.
		transactions, err := server.db.FindTransactionsByRenter(renter.ID)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		json.NewEncoder(w).Encode(transactions)
	})
}

func (server *MetaServer) getProviderTransactionsHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if providerID, present := claims["providerID"]; !present || providerID.(string) != params["providerID"] {
			writeErr("cannot access other users' transactions", http.StatusUnauthorized, w)
			return
		}

		// Make sure the specified provider actually exists.
		provider, err := server.db.FindProviderByID(params["providerID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		// Retrieve the provider's transactions.
		transactions, err := server.db.FindTransactionsByProvider(provider.ID)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		json.NewEncoder(w).Encode(transactions)
	})
}
