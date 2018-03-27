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
			writeErr("cannot withdraw form other users", http.StatusUnauthorized, w)
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

type PaypalWithdrawReq struct {
	Amount   float64 `json:"amount"`
	Email    string  `json:"email"`
	RenterID string  `json:"renterID"`
}

func (server *MetaServer) getPaypalWithdrawHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload PaypalWithdrawReq
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			writeErr("Could not parse payload", http.StatusBadRequest, w)
			return
		}

		if payload.Email == "" {
			writeErr("Must supply email", http.StatusBadRequest, w)
			return
		}

		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != payload.RenterID {
			writeErr("cannot withdraw form other users", http.StatusUnauthorized, w)
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

		// Fail if the user tries to withdraw more than they currently have.
		if payload.Amount > renter.Wallet.Balance {
			writeErr("Cannot withdraw more than balance", http.StatusBadRequest, w)
			return
		}

		_, err = c.CreateSinglePayout(paypalsdk.Payout{
			SenderBatchHeader: &paypalsdk.SenderBatchHeader{
				EmailSubject: "Withdrawal from SkyBin",
			},
			Items: []paypalsdk.PayoutItem{
				paypalsdk.PayoutItem{
					RecipientType: "EMAIL",
					Receiver:      payload.Email,
					Amount: &paypalsdk.AmountPayout{
						Currency: "USD",
						Value:    fmt.Sprintf("%.2f", payload.Amount),
					},
					Note: fmt.Sprintf("Renter ID: %s", renter.ID),
				},
			},
		})
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			errorResp := err.(*paypalsdk.ErrorResponse)
			server.logger.Printf("%+v", errorResp.Details)
			return
		}

		// BUG(kincaid): Possible race condition here. Add DB operation for atomically decrementing wallet balance.
		renter.Wallet.Balance -= payload.Amount
		err = server.db.UpdateRenter(renter)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
