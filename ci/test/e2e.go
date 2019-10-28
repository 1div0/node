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

package test

import (
	"fmt"
	"time"

	"github.com/magefile/mage/sh"
	"github.com/mysteriumnetwork/node/logconfig"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type runner struct {
	compose         func(args ...string) error
	etherPassphrase string
	testEnv         string
	services        string
}

// TestE2EBasic runs end-to-end tests
func TestE2EBasic() error {
	logconfig.Bootstrap()
	composeFiles := []string{
		"bin/localnet/docker-compose.yml",
		"e2e/docker-compose.yml",
	}
	runner := newRunner(composeFiles, "node_e2e_basic_test", "openvpn,noop,wireguard")
	defer runner.cleanup()
	if err := runner.init(); err != nil {
		return err
	}

	return runner.test()
}

// TestE2ENAT runs end-to-end tests in NAT environment
func TestE2ENAT() error {
	logconfig.Bootstrap()
	composeFiles := []string{
		"e2e/traversal/docker-compose.yml",
	}
	runner := newRunner(composeFiles, "node_e2e_nat_test", "openvpn")
	defer runner.cleanup()
	if err := runner.init(); err != nil {
		return err
	}

	return runner.test()
}

// TODO this can be merged into runner.test() itself
func (r *runner) init() error {
	if err := r.startAppContainers(); err != nil {
		return err
	}

	if err := r.startProviderConsumerNodes(); err != nil {
		return err
	}
	return nil
}

func (r *runner) startAppContainers() error {
	log.Info().Msg("Starting other services")
	if err := r.compose("up", "-d", "broker", "ganache", "ipify"); err != nil {
		return errors.Wrap(err, "starting other services failed!")
	}
	log.Info().Msg("Starting DB")
	if err := r.compose("up", "-d", "db"); err != nil {
		return errors.Wrap(err, "starting DB failed!")
	}

	dbUp := false
	for start := time.Now(); !dbUp && time.Since(start) < 60*time.Second; {
		err := r.compose("exec", "-T", "db", "mysqladmin", "ping", "--protocol=TCP", "--silent")
		if err != nil {
			log.Info().Msg("Waiting...")
		} else {
			log.Info().Msg("DB is up")
			dbUp = true
			break
		}
	}
	if !dbUp {
		return errors.New("starting DB timed out")
	}

	log.Info().Msg("Starting transactor")
	if err := r.compose("up", "-d", "transactor"); err != nil {
		return errors.Wrap(err, "starting transactor failed!")
	}

	log.Info().Msg("Migrating DB")
	if err := r.compose("run", "--entrypoint", "bin/db-upgrade", "mysterium-api"); err != nil {
		return errors.Wrap(err, "migrating DB failed!")
	}

	log.Info().Msg("Starting mysterium-api")
	if err := r.compose("up", "-d", "mysterium-api"); err != nil {
		return errors.Wrap(err, "starting mysterium-api failed!")
	}

	log.Info().Msg("Deploying contracts")
	err := r.compose("run", "go-runner",
		"go", "run", "bin/localnet/deployer/deployer.go",
		"--keystore.directory=bin/localnet/deployer/keystore",
		"--ether.address=0x354Bd098B4eF8c9E70B7F21BE2d455DF559705d7",
		fmt.Sprintf("--ether.passphrase=%v", r.etherPassphrase),
		"--geth.url=ws://ganache:8545")
	if err != nil {
		return errors.Wrap(err, "failed to deploy contracts!")
	}

	log.Info().Msg("starting accountant")
	if err := r.compose("up", "-d", "accountant"); err != nil {
		return errors.Wrap(err, "starting accountant failed!")
	}

	return nil
}

func (r *runner) startProviderConsumerNodes() error {
	log.Info().Msg("Building app images")
	if err := r.compose("build"); err != nil {
		return errors.Wrap(err, "building app images failed!")
	}

	log.Info().Msg("Starting app containers")
	if err := r.compose("up", "-d", "myst-provider", "myst-consumer"); err != nil {
		return errors.Wrap(err, "starting app containers failed!")
	}
	return nil
}

func (r *runner) test() error {
	log.Info().Msg("Running tests for env: " + r.testEnv)

	err := r.compose("run", "go-runner",
		"go", "test", "-v", "./e2e/...", "-args",
		"--deployer.keystore-directory=../bin/localnet/deployer/keystore",
		"--deployer.address=0x354Bd098B4eF8c9E70B7F21BE2d455DF559705d7",
		"--deployer.passphrase", r.etherPassphrase,
		"--provider.tequilapi-host=myst-provider",
		"--provider.tequilapi-port=4050",
		"--consumer.tequilapi-host=myst-consumer",
		"--consumer.tequilapi-port=4050",
		"--geth.url=ws://ganache:8545",
		"--consumer.services", r.services,
	)
	return errors.Wrap(err, "tests failed!")
}

func (r *runner) cleanup() {
	log.Info().Msg("Cleaning up")
	_ = r.compose("logs")
	if err := r.compose("down", "--volumes", "--remove-orphans", "--timeout", "30"); err != nil {
		log.Warn().Err(err).Msg("Cleanup error")
	}
}

func newRunner(composeFiles []string, testEnv, services string) *runner {
	fileArgs := make([]string, 0)
	for _, f := range composeFiles {
		fileArgs = append(fileArgs, "-f", f)
	}
	var args []string
	args = append(args, fileArgs...)
	args = append(args, "-p", testEnv)

	return &runner{
		compose:  sh.RunCmd("docker-compose", args...),
		testEnv:  testEnv,
		services: services,
	}
}
