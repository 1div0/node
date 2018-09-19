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

package discovery

import (
	"testing"
	"time"

	"github.com/mysteriumnetwork/node/identity"
	identity_registry "github.com/mysteriumnetwork/node/identity/registry"
	"github.com/mysteriumnetwork/node/service_discovery/dto"
	"github.com/stretchr/testify/assert"
)

var (
	providerID = identity.FromAddress("my-identity")
	proposal   = dto.ServiceProposal{
		ProviderID: providerID.Address,
	}
)

func TestStartRegistersProposal(t *testing.T) {
	d := NewFakeDiscovery()
	d.identityRegistry = &identity_registry.FakeRegistry{RegistrationEventExists: false, Registered: true}

	d.Start(providerID, proposal)

	actualStatus := observeStatus(d, PingProposal)
	assert.Equal(t, PingProposal, actualStatus)
}

func TestStartRegistersIdentitySuccessfully(t *testing.T) {
	d := NewFakeDiscovery()
	d.identityRegistry = &identity_registry.FakeRegistry{RegistrationEventExists: true, Registered: false}

	d.Start(providerID, proposal)

	actualStatus := observeStatus(d, PingProposal)
	assert.Equal(t, PingProposal, actualStatus)
}

func TestStartRegisterIdentityCancelled(t *testing.T) {
	d := NewFakeDiscovery()
	d.identityRegistry = &identity_registry.FakeRegistry{RegistrationEventExists: false, Registered: false}

	d.Start(providerID, proposal)

	actualStatus := observeStatus(d, WaitingForRegistration)
	assert.Equal(t, WaitingForRegistration, actualStatus)

	d.Stop()

	actualStatus = observeStatus(d, IdentityRegisterFailed)
	assert.Equal(t, IdentityRegisterFailed, actualStatus)
}

func TestStartStopUnregisterProposal(t *testing.T) {
	d := NewFakeDiscovery()
	d.identityRegistry = &identity_registry.FakeRegistry{RegistrationEventExists: false, Registered: true}

	d.Start(providerID, proposal)

	actualStatus := observeStatus(d, PingProposal)
	assert.Equal(t, PingProposal, actualStatus)

	d.Stop()

	actualStatus = observeStatus(d, ProposalUnregistered)
	assert.Equal(t, ProposalUnregistered, actualStatus)
}

func observeStatus(d *Discovery, status Status) Status {
	for {
		d.RLock()
		if d.status == status {
			d.RUnlock()
			return d.status
		}
		d.RUnlock()
		time.Sleep(10 * time.Millisecond)
	}
}
