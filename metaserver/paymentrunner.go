package metaserver

import (
	"fmt"
	"skybin/core"
	"time"
)

func (server *MetaServer) startPaymentRunner() {
	// Frequency at which the runner should be triggered (should probably put this in a config file)
	runnerFrequency := time.Minute * 1

	// Ticker triggering the runner.
	ticker := time.NewTicker(runnerFrequency)

	go func() {
		for range ticker.C {
			server.logger.Println("Running payments...")
			err := server.runPayments()
			if err != nil {
				server.logger.Println("Error when running payments:", err)
			}
		}
	}()
}

func (server *MetaServer) runPayments() error {
	// Retrieve a list of all contracts on the server.
	contracts, err := server.db.FindAllContracts()
	if err != nil {
		return err
	}

	// Retrieve a list of all the payments on the server.
	payments, err := server.db.FindAllPayments()
	if err != nil {
		return err
	}

	// Put the payments in a map so we can retrieve them a bit faster.
	contractToPayment := make(map[string]core.PaymentInfo)
	for _, item := range payments {
		contractToPayment[item.ContractID] = item
	}

	// For each contract...
	for _, item := range contracts {
		// server.logger.Println("Payment for contract", item.ID)

		// Payment information associated with this contract.
		paymentInfo := contractToPayment[item.ID]

		// If we're not currently paying down the contract, skip it.
		if !paymentInfo.IsPaying {
			continue
		}

		// Determine the portion of the contract price that should be paid.
		totalContractTime := item.EndDate.Sub(item.StartDate)
		timeSinceLastPayment := time.Now().Sub(paymentInfo.LastPaymentTime)
		portionToPay := float64(timeSinceLastPayment.Nanoseconds()) / float64(totalContractTime.Nanoseconds())
		amountToPay := int64(portionToPay * float64(item.StorageFee))
		// server.logger.Println("Portion to pay:", portionToPay, "Amount:", amountToPay)

		// If the amount to pay is 0, we probably don't have anything to pay
		// due to integer truncation and floating point errors (basically, we
		// don't have enough to pay), so we should just wait until we do.
		if amountToPay == 0 {
			continue
		}

		// If that portion is greater than the remaining contract balance, just pay the remaining balance.
		if amountToPay > paymentInfo.Balance {
			amountToPay = paymentInfo.Balance
		}

		// Transfer the amount from the contract to the provider.
		// TODO: Again, we should add atomic increment and decrement db methods so race conditions don't happen.
		provider, err := server.db.FindProviderByID(item.ProviderId)
		if err != nil {
			return err
		}
		provider.Balance += amountToPay
		err = server.db.UpdateProvider(provider)
		if err != nil {
			return err
		}
		paymentInfo.Balance -= amountToPay
		paymentInfo.LastPaymentTime = time.Now()

		// If the balance is now 0, mark the contract as not being paid.
		if paymentInfo.Balance == 0 {
			paymentInfo.IsPaying = false
		}

		// Update the payment information.
		err = server.db.UpdatePayment(&paymentInfo)
		if err != nil {
			return err
		}

		// Create a transaction showing the payment.
		transaction := &core.Transaction{
			UserType:        "provider",
			UserID:          provider.ID,
			ContractID:      item.ID,
			TransactionType: "payment",
			Amount:          amountToPay,
			Date:            time.Now(),
			Description:     fmt.Sprintf("Payment for contract %s", item.ID),
		}
		err = server.db.InsertTransaction(transaction)
		if err != nil {
			return err
		}
	}

	return nil
}
