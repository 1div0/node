/*
 * Copyright (C) 2021 The "MysteriumNetwork/node" Authors.
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

package proposal

import (
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/mysteriumnetwork/node/market"

	"github.com/pkg/errors"
)

var errMsgBoltNotFound = "not found"

type persistentStorage interface {
	Store(bucket string, data interface{}) error
	GetAllFrom(bucket string, data interface{}) error
	GetLast(bucket string, to interface{}) error
	Delete(bucket string, data interface{}) error
}

const (
	bucketName = "proposal-filter-presets"
	startingID = 100
)

// FilterPresetStorage filter preset storage
type FilterPresetStorage struct {
	lock    sync.Mutex
	storage persistentStorage
}

// NewFilterPresetStorage constructor for FilterPresetStorage
func NewFilterPresetStorage(storage persistentStorage) *FilterPresetStorage {
	return &FilterPresetStorage{
		storage: storage,
	}
}

// List list all filter presets
// system preset are identified by preset.ID < startingID
func (fps *FilterPresetStorage) List() (*FilterPresets, error) {
	fps.lock.Lock()
	defer fps.lock.Unlock()

	entries, err := fps.ls()
	return filterPresets(entries).prependSystemPresets(), err
}

// Get get single filter preset
func (fps *FilterPresetStorage) Get(id int) (*FilterPreset, error) {
	fps.lock.Lock()
	defer fps.lock.Unlock()

	entries, err := fps.ls()
	if err != nil {
		return nil, errors.New("failed to load filter presets")
	}
	byId, ok := filterPresets(entries).prependSystemPresets().byId(id)
	if !ok {
		return nil, fmt.Errorf("filter preset id: %d not found", id)
	}
	return &byId, nil
}

func (fps *FilterPresetStorage) ls() ([]FilterPreset, error) {
	var entries []FilterPreset
	err := fps.storage.GetAllFrom(bucketName, &entries)
	return entries, err
}

// Save created or updates existing
// to update existing: preset.ID > startingID
func (fps *FilterPresetStorage) Save(preset FilterPreset) error {
	fps.lock.Lock()
	defer fps.lock.Unlock()

	if preset.ID != 0 {
		return fps.storage.Store(bucketName, &preset)
	}

	nextID, err := fps.nextID()
	if err != nil {
		return err
	}

	preset.ID = nextID
	err = fps.storage.Store(bucketName, &preset)
	if err != nil {
		return err
	}

	return nil
}

// Delete delete filter preset by id
func (fps *FilterPresetStorage) Delete(id int) error {
	fps.lock.Lock()
	defer fps.lock.Unlock()

	if id < 100 {
		return errors.New("deleting system presets is not allowed")
	}

	toRemove := FilterPreset{ID: id}
	return fps.storage.Delete(bucketName, &toRemove)
}

func (fps *FilterPresetStorage) nextID() (int, error) {
	var last FilterPreset
	err := fps.storage.GetLast(bucketName, &last)
	if err != nil {
		if err.Error() == errMsgBoltNotFound {
			return startingID, nil
		}
		return 0, err
	}
	return last.ID + 1, err
}

var defaultPresets = []FilterPreset{
	{
		ID:     1,
		Name:   "Media Streaming",
		IPType: Residential,
		filter: func(proposals []market.ServiceProposal) []market.ServiceProposal {
			var totalBandwidth, averageBandwidth float64
			var totalQuality, avgQuality float64

			for _, p := range proposals {
				totalBandwidth += p.Quality.Bandwidth
				totalQuality += p.Quality.Quality
			}
			averageBandwidth = totalBandwidth / float64(len(proposals))
			avgQuality = totalQuality / float64(len(proposals))

			var filtered []market.ServiceProposal
			for _, p := range proposals {
				if p.Quality.Quality >= avgQuality && p.Quality.Bandwidth >= averageBandwidth && p.Location.IPType == "residential" {
					filtered = append(filtered, p)
				}
			}

			sort.SliceStable(filtered, func(i, j int) bool {
				qx, qy := filtered[i].Quality, filtered[j].Quality

				if qx.Bandwidth == qy.Bandwidth {
					return qx.Quality > qy.Quality
				}
				return qx.Bandwidth > qy.Bandwidth
			})

			return filtered
		},
	},
	{
		ID:   2,
		Name: "Browsing",
		filter: func(proposals []market.ServiceProposal) []market.ServiceProposal {
			totalPerHour, totalPerGiB := new(big.Int), new(big.Int)
			avgPerHour, avgPerGiB := new(big.Int), new(big.Int)
			var totalQuality, avgQuality float64

			for _, p := range proposals {
				totalPerHour = new(big.Int).Add(totalPerHour, p.Price.PerHour)
				totalPerGiB = new(big.Int).Add(totalPerGiB, p.Price.PerGiB)
				totalQuality += p.Quality.Quality
			}
			avgPerHour = new(big.Int).Sub(totalPerHour, avgPerHour)
			avgPerGiB = new(big.Int).Sub(totalPerGiB, avgPerGiB)
			avgQuality = totalQuality / float64(len(proposals))

			var filtered []market.ServiceProposal
			for _, p := range proposals {
				if p.Price.PerGiB.Cmp(avgPerGiB) <= 0 && p.Price.PerHour.Cmp(avgPerHour) <= 0 && p.Quality.Quality > avgQuality {
					filtered = append(filtered, p)
				}
			}
			sort.SliceStable(filtered, func(i, j int) bool {
				qx, qy := filtered[i].Quality, filtered[j].Quality
				px, py := filtered[i].Price, filtered[j].Price
				if qx.Quality == qy.Quality {
					if px.PerGiB.Cmp(py.PerGiB) == 0 {
						return px.PerHour.Cmp(py.PerHour) == -1
					}
					return px.PerGiB.Cmp(py.PerGiB) == -1
				}
				return qx.Quality > qy.Quality
			})

			return filtered
		},
	},
	{
		ID:     3,
		Name:   "Download",
		IPType: Hosting,
		filter: func(proposals []market.ServiceProposal) []market.ServiceProposal {
			totalPerHour, totalPerGiB := new(big.Int), new(big.Int)
			avgPerHour, avgPerGiB := new(big.Int), new(big.Int)

			for _, p := range proposals {
				totalPerHour = new(big.Int).Add(totalPerHour, p.Price.PerHour)
				totalPerGiB = new(big.Int).Add(totalPerGiB, p.Price.PerGiB)
			}
			avgPerHour = new(big.Int).Sub(totalPerHour, avgPerHour)
			avgPerGiB = new(big.Int).Sub(totalPerGiB, avgPerGiB)

			var filtered []market.ServiceProposal
			for _, p := range proposals {
				if p.Price.PerGiB.Cmp(avgPerGiB) <= 0 && p.Price.PerHour.Cmp(avgPerHour) <= 0 && p.Location.IPType == "hosting" {
					filtered = append(filtered, p)
				}
			}
			sort.SliceStable(filtered, func(i, j int) bool {
				px, py := filtered[i].Price, filtered[j].Price
				if px.PerGiB.Cmp(py.PerGiB) == 0 {
					return px.PerHour.Cmp(py.PerHour) == -1
				}
				return px.PerGiB.Cmp(py.PerGiB) == -1
			})

			return filtered
		},
	},
}

// IPType represents type of node
type IPType string

const (
	// Residential node type value
	Residential IPType = "residential"
	// Hosting node type value
	Hosting IPType = "hosting"
	// Business node type value
	Business IPType = "business"
	// Cellular node type value
	Cellular IPType = "cellular"
	// Dialup node type value
	Dialup IPType = "dialup"
	// College node type value
	College IPType = "college"
)

// FilterPreset represent predefined or user stored proposal filter preset
type FilterPreset struct {
	ID     int
	Name   string
	IPType IPType
	filter func(proposals []market.ServiceProposal) []market.ServiceProposal
}

// Filter filters proposals according to preset
func (fps *FilterPreset) Filter(proposals []market.ServiceProposal) []market.ServiceProposal {
	return fps.filter(proposals) // because of storage, fps.filter can't be exported as a struct property
}

func filterPresets(entries []FilterPreset) *FilterPresets {
	return &FilterPresets{Entries: entries}
}

// FilterPresets convenience wrapper
type FilterPresets struct {
	Entries []FilterPreset
}

func (ls *FilterPresets) prependSystemPresets() *FilterPresets {
	var result = make([]FilterPreset, len(defaultPresets))
	copy(result, defaultPresets)
	ls.Entries = append(result, ls.Entries...)
	return ls
}

func (ls *FilterPresets) byId(id int) (FilterPreset, bool) {
	for _, e := range ls.Entries {
		if e.ID == id {
			return e, true
		}
	}

	return FilterPreset{}, false
}
