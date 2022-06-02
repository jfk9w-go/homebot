package tinkoff

import (
	"testing"

	"github.com/jfk9w-go/flu/httpf"
)

func TestConfirm(t *testing.T) {
	confirm := confirm{
		confirmationData: confirmationData{
			SMSBYID: "123",
		},
		initialOperation:       "sign_up",
		initialOperationTicket: "5666",
	}

	form := httpf.FormValue(confirm)
	println(form)
}
