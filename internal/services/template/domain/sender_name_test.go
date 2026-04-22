package domain

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSenderName_RejectsSpaces(t *testing.T) {
	err := ValidateSenderName("My Brand")
	assert.ErrorIs(t, err, ErrInvalidSenderNameFormat)
}

func TestValidateSenderName_AcceptsDotUnderscoreDash(t *testing.T) {
	for _, c := range []string{"My.Brand", "my_brand", "My-Brand", "Shop1.2"} {
		t.Run(c, func(t *testing.T) {
			assert.NoError(t, ValidateSenderName(c))
		})
	}
}

func TestValidateSenderName_AcceptsAlphanumeric11(t *testing.T) {
	assert.NoError(t, ValidateSenderName("MyBrand1"))
	assert.NoError(t, ValidateSenderName("MyBrand0123"))
}

func TestValidateSenderName_AcceptsNumeric15(t *testing.T) {
	assert.NoError(t, ValidateSenderName("123456789012345"))
}

func TestValidateSenderName_RejectsTooLongAlpha(t *testing.T) {
	assert.ErrorIs(t, ValidateSenderName("MyVeryLongBrand"), ErrInvalidSenderNameFormat)
}

func TestValidateSenderName_RejectsEmpty(t *testing.T) {
	assert.ErrorIs(t, ValidateSenderName(""), ErrInvalidSenderNameFormat)
}

func TestValidateChannel_Valid(t *testing.T) {
	for _, c := range []string{SenderNameChannelSMS, SenderNameChannelVoice, SenderNameChannelViber} {
		t.Run(c, func(t *testing.T) {
			assert.NoError(t, ValidateChannel(c))
		})
	}
}

func TestValidateChannel_Invalid(t *testing.T) {
	cases := []string{"", "email", "SMS", "Voice", " sms ", "mms"}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q", c), func(t *testing.T) {
			assert.ErrorIs(t, ValidateChannel(c), ErrInvalidChannel)
		})
	}
}
