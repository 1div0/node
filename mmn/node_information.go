/*
 * Copyright (C) 2019 The "MysteriumNetwork/node" Authors.
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

package mmn

import (
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"strings"

	"github.com/mysteriumnetwork/go-ci/shell"
	"github.com/pkg/errors"

	"github.com/mysteriumnetwork/node/core/ip"
	"github.com/mysteriumnetwork/node/metadata"
)

// NodeInformation contains node information to be sent to MMN
type NodeInformation struct {
	MACAddress  string `json:"mac_address"` // SHA256 hash
	LocalIP     string `json:"local_ip"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	NodeVersion string `json:"node_version"`
	Identity    string `json:"identity"`
}

// CollectNodeData sends node information to MMN on identity unlock
func CollectNodeData(client *client, resolver ip.Resolver) func(string) {
	return func(identity string) {
		outboundIp, err := resolver.GetOutboundIPAsString()
		if err != nil {
			log.Error(errors.Wrap(err, "failed to get Outbound IP"))
		}

		mac, err := ip.GetMACAddressForIP(outboundIp)
		if err != nil {
			log.Error(errors.Wrap(err, "failed to MAC address"))
		}

		info := getNodeInformation()
		if err != nil {
			log.Error(errors.Wrap(err, "failed to get NodeInformation for MMN"))
			return
		}

		sha256Bytes := sha256.Sum256([]byte(mac))

		info.MACAddress = hex.EncodeToString(sha256Bytes[:])
		info.LocalIP = outboundIp
		info.Identity = identity

		if err = client.RegisterNode(info); err != nil {
			log.Error(errors.Wrap(err, "failed to send NodeInformation to MMN"))
		}

		log.Info("Registered node to my.mysterium.network")
	}
}

func getNodeInformation() *NodeInformation {
	info := &NodeInformation{
		Arch:        runtime.GOOS + "/" + runtime.GOARCH,
		OS:          getOS(),
		NodeVersion: metadata.VersionAsString(),
	}

	return info
}

func getOS() string {
	switch runtime.GOOS {
	case "darwin":
		output, err := shell.NewCmd("sw_vers -productVersion").Output()
		if err != nil {
			log.Error(errors.Wrap(err, "failed to get OS information"))
			return ""
		}
		return "MAC OS X - " + strings.TrimSpace(string(output))

	case "linux":
		output, err := shell.NewCmd("lsb_release -d").Output()
		if err != nil {
			log.Error(errors.Wrap(err, "failed to get OS information"))
			return ""
		}
		return strings.TrimSpace(strings.Replace(string(output), "Description:", "", 1))
	}

	return ""
}
