package metaserver

import (
	"encoding/json"
	"net/http"

	"github.com/logpacker/PayPal-Go-SDK"
)

type CreatePaypalPaymentReq struct {
	Amount int `json:"amount"`
}

type CreatePaypalPaymentResp struct {
	ID string `json:"id"`
}

func (server *MetaServer) getCreatePaypalPaymentHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
					Total:    "7.00",
				},
				Description: "My Payment",
			}},
			RedirectURLs: &paypalsdk.RedirectURLs{
				ReturnURL: "/my-wallet",
				CancelURL: "/my-wallet",
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
