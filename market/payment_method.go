/*
 * Copyright (C) 2017 The "MysteriumNetwork/node" Authors.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package market

import (
	"encoding/json"
	"time"

	"github.com/mysteriumnetwork/node/money"
	"github.com/rs/zerolog/log"
)

// PaymentMethod is a interface for all types of payment methods
type PaymentMethod interface {
	// Service price per unit of metering
	GetPrice() money.Money
	GetType() string
	GetRate() PaymentRate
}

// PaymentRate represents the payment rate
type PaymentRate struct {
	PerTime time.Duration
	PerByte uint64
}

// UnsupportedPaymentMethod represents payment method which is unknown to node (i.e. not registered)
type UnsupportedPaymentMethod struct {
}

// GetPrice should not be called
func (UnsupportedPaymentMethod) GetPrice() money.Money {
	//this should never be called
	log.Error().Msg("Unsupported proposal GetPrice should not be called")
	return money.Money{}
}

// GetType should not be called
func (UnsupportedPaymentMethod) GetType() string {
	//this should never be called
	log.Error().Msg("Unsupported proposal GetType should not be called")
	return ""
}

// GetRate should not be called
func (UnsupportedPaymentMethod) GetRate() PaymentRate {
	//this should never be called
	log.Error().Msg("Unsupported proposal GetRate should not be called")
	return PaymentRate{}
}

var _ PaymentMethod = UnsupportedPaymentMethod{}

// PaymentMethodUnserializer is function type which takes raw json message and returns deserialized payment method
type PaymentMethodUnserializer func(*json.RawMessage) (PaymentMethod, error)

// service payment method unserializer registry
//TODO same idea as for contact global map
var paymentMethodMap = make(map[string]PaymentMethodUnserializer)

// RegisterPaymentMethodUnserializer registers unserializer for specified payment method type
func RegisterPaymentMethodUnserializer(paymentMethod string, unserializer func(*json.RawMessage) (PaymentMethod, error)) {
	paymentMethodMap[paymentMethod] = unserializer
}

func unserializePaymentMethod(paymentMethod string, message *json.RawMessage) PaymentMethod {
	method, ok := paymentMethodMap[paymentMethod]
	if !ok {
		return UnsupportedPaymentMethod{}
	}
	pm, err := method(message)
	if err != nil {
		return UnsupportedPaymentMethod{}
	}
	return pm
}
