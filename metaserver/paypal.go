package metaserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"skybin/util"
	"strconv"

	"github.com/logpacker/PayPal-Go-SDK"
)

type CreatePaypalPaymentReq struct {
	Amount    float64 `json:"amount"`
	ReturnURL string  `json:"returnURL"`
	CancelURL string  `json:"cancelURL"`
}

type CreatePaypalPaymentResp struct {
	ID string `json:"id"`
}

func (server *MetaServer) getCreatePaypalPaymentHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Perform validation on the amount we recieve.
		var payload CreatePaypalPaymentReq
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			writeErr("Could not parse payload", http.StatusBadRequest, w)
			return
		}

		c, err := paypalsdk.NewClient(
			"AVxS4Zhi1bwj9ahQx_Rx6x99blBFPNkUkMPOGOxLVGhl3mwjzxJ1RuW_eIqyO7DWempaJLKleD267Jqo",
			"EEWY_gYzFbIh4xduZ5t-AtuexnSBwgdzA7FmaDeUKF1qAKlgX2RoTFaTRSfD5UUVwuSXSvJrxCUog3cw",
			paypalsdk.APIBaseSandBox,
		)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		_, err = c.GetAccessToken()
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		p := paypalsdk.Payment{
			Intent: "sale",
			Payer: &paypalsdk.Payer{
				PaymentMethod: "paypal",
			},
			Transactions: []paypalsdk.Transaction{{
				Amount: &paypalsdk.Amount{
					Currency: "USD",
					Total:    fmt.Sprintf("%.2f", payload.Amount),
				},
				Description: "My Payment",
			}},
			RedirectURLs: &paypalsdk.RedirectURLs{
				ReturnURL: payload.ReturnURL,
				CancelURL: payload.CancelURL,
			},
		}
		resp, err := c.CreatePayment(p)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			errorResp := err.(*paypalsdk.ErrorResponse)
			server.logger.Printf("%+v", errorResp.Details)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(CreatePaypalPaymentResp{ID: resp.ID})
		server.logger.Println("it worked")
	})
}

type PaypalExecuteReq struct {
	PaymentID string `json:"paymentID"`
	PayerID   string `json:"payerID"`
	RenterID  string `json:"renterID"`
}

func (server *MetaServer) getExecutePaypalPaymentHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload PaypalExecuteReq
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			writeErr("Could not parse payload", http.StatusBadRequest, w)
			return
		}

		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != payload.RenterID {
			writeErr("cannot deposit in other users' accounts", http.StatusUnauthorized, w)
			return
		}

		renter, err := server.db.FindRenterByID(payload.RenterID)
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}

		c, err := paypalsdk.NewClient(
			"AVxS4Zhi1bwj9ahQx_Rx6x99blBFPNkUkMPOGOxLVGhl3mwjzxJ1RuW_eIqyO7DWempaJLKleD267Jqo",
			"EEWY_gYzFbIh4xduZ5t-AtuexnSBwgdzA7FmaDeUKF1qAKlgX2RoTFaTRSfD5UUVwuSXSvJrxCUog3cw",
			paypalsdk.APIBaseSandBox,
		)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		_, err = c.GetAccessToken()
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		resp, err := c.ExecuteApprovedPayment(payload.PaymentID, payload.PayerID)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			errorResp := err.(*paypalsdk.ErrorResponse)
			server.logger.Printf("%+v", errorResp.Details)
			return
		}

		// BUG(kincaid): Possible race condition here. Add DB operation for atomically incrementing wallet balance.
		server.logger.Println(resp.Transactions[0].Amount.Total)
		amount, err := strconv.ParseFloat(resp.Transactions[0].Amount.Total, 64)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		renter.Wallet.Balance += amount
		err = server.db.UpdateRenter(renter)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
