package metaserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"skybin/core"
	"skybin/util"
	"strconv"
	"strings"
	"time"

	"github.com/logpacker/PayPal-Go-SDK"
)

type CreatePaypalPaymentReq struct {
	// Amount for the payment, in cents.
	Amount    int64  `json:"amount"`
	ReturnURL string `json:"returnURL"`
	CancelURL string `json:"cancelURL"`
}

type CreatePaypalPaymentResp struct {
	ID string `json:"id"`
}

func (server *MetaServer) getCreatePaypalPaymentHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Perform validation on the amount we recieve.
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

		dollars := payload.Amount / 100
		cents := payload.Amount % 100
		amount := fmt.Sprintf("%d.%d", dollars, cents)

		p := paypalsdk.Payment{
			Intent: "sale",
			Payer: &paypalsdk.Payer{
				PaymentMethod: "paypal",
			},
			Transactions: []paypalsdk.Transaction{{
				Amount: &paypalsdk.Amount{
					Currency: "USD",
					Total:    amount,
				},
				Description: "SkyBin deposit",
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
		amountInCents, err := strconv.ParseInt(
			strings.Replace(resp.Transactions[0].Amount.Total, ".", "", 1),
			10,
			64,
		)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		// The renter balance is in tenths of cents, so convert accordingly.
		renter.Balance += amountInCents * 10
		err = server.db.UpdateRenter(renter)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		// Create a transaction showing the deposit.
		transaction := &core.Transaction{
			UserType:        "renter",
			UserID:          renter.ID,
			TransactionType: "deposit",
			Amount:          amountInCents * 10,
			Date:            time.Now(),
			Description:     "Paypal deposit",
		}
		err = server.db.InsertTransaction(transaction)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

type RenterPaypalWithdrawReq struct {
	// Amount to withdraw in cents.
	Amount   int64  `json:"amount"`
	Email    string `json:"email"`
	RenterID string `json:"renterID"`
}

func (server *MetaServer) getRenterPaypalWithdrawHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload RenterPaypalWithdrawReq
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
		if payload.Amount*10 > renter.Balance {
			writeErr("Cannot withdraw more than balance", http.StatusBadRequest, w)
			return
		}

		dollars := payload.Amount / 100
		cents := payload.Amount % 100
		amountToWithdraw := fmt.Sprintf("%d.%d", dollars, cents)

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
						Value:    amountToWithdraw,
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
		renter.Balance -= payload.Amount * 10
		err = server.db.UpdateRenter(renter)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		// Create a transaction showing the withdrawal.
		transaction := &core.Transaction{
			UserType:        "renter",
			UserID:          renter.ID,
			TransactionType: "withdrawal",
			Amount:          payload.Amount * 10,
			Date:            time.Now(),
			Description:     "Withdrawal",
		}
		err = server.db.InsertTransaction(transaction)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

type ProviderPaypalWithdrawReq struct {
	// Amount to withdraw in cents.
	Amount     int64  `json:"amount"`
	Email      string `json:"email"`
	ProviderID string `json:"providerID"`
}

func (server *MetaServer) getProviderPaypalWithdrawHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload ProviderPaypalWithdrawReq
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
		if providerID, present := claims["providerID"]; !present || providerID.(string) != payload.ProviderID {
			writeErr("cannot withdraw form other users", http.StatusUnauthorized, w)
			return
		}

		provider, err := server.db.FindProviderByID(payload.ProviderID)
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
		if payload.Amount*10 > provider.Balance {
			writeErr("Cannot withdraw more than balance", http.StatusBadRequest, w)
			return
		}

		dollars := payload.Amount / 100
		cents := payload.Amount % 100
		amountToWithdraw := fmt.Sprintf("%d.%d", dollars, cents)

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
						Value:    amountToWithdraw,
					},
					Note: fmt.Sprintf("Provider ID: %s", provider.ID),
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
		provider.Balance -= payload.Amount * 10
		err = server.db.UpdateProvider(provider)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		// Create a transaction showing the withdrawal.
		transaction := &core.Transaction{
			UserType:        "provider",
			UserID:          provider.ID,
			TransactionType: "withdrawal",
			Amount:          payload.Amount * 10,
			Date:            time.Now(),
			Description:     "Withdrawal",
		}
		err = server.db.InsertTransaction(transaction)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
