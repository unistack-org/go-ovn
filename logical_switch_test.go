/**
 * Copyright (c) 2019 eBay Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 **/

package goovn

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	LS3             = "LS3"
	NEUTRON_NETWORK = "neutron:network"
	DUMMY           = "dummy"
	FOO             = "foo"
	BAR             = "bar"
)

func TestLogicalSwitchAdd(t *testing.T) {
	t.Logf("Add LogicalSwitch")
	cmd, err := ovndbapi.LogicalSwitch.Add(LogicalSwitchName(LS3))
	if err != nil {
		t.Fatal(err)
	}
	err = ovndbapi.Execute(cmd)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("List LogicalSwitch")
	lsList, err := ovndbapi.LogicalSwitch.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(lsList) != 1 {
		t.Fatalf("invalid ls count found %#v\n", lsList)
	}
}

func TestLogicalSwitchGet(t *testing.T) {
	t.Logf("Get LogicalSwitch")
	ls, err := ovndbapi.LogicalSwitch.Get(LogicalSwitchName(LS3))
	if err != nil {
		t.Fatal(err)
	}
	if ls == nil || ls.Name != LS3 {
		t.Fatalf("logical switch %s not found: %v", LS3, ls)
	}
}

func TestLogicalSwitchSetExternalIDs(t *testing.T) {
	t.Logf("SetExternalIDs LogicalSwitch")
	cmd, err := ovndbapi.LogicalSwitch.SetExternalIDs(
		LogicalSwitchName(LS3),
		LogicalSwitchExternalIDs(map[string]string{NEUTRON_NETWORK: DUMMY, FOO: BAR}))
	if err != nil {
		t.Fatal(err)
	}
	err = ovndbapi.Execute(cmd)
	if err != nil {
		t.Fatal(err)
	}
	// Get LS3 and get external_id NEUTRON_NETWORK
	ls, err := ovndbapi.LogicalSwitch.Get(LogicalSwitchName(LS3))
	if err != nil {
		t.Fatal(err)
	}
	for key, val := range ls.ExternalIDs {
		if key == NEUTRON_NETWORK {
			assert.Equal(t, true, val == DUMMY, "Got external ID dummy")
			t.Logf("Successfully validated external_id key NEUTRON_NETWORK to LS3")
		}
	}
	// Add empty external_ids to LS3
	cmd, err = ovndbapi.LogicalSwitch.SetExternalIDs(LogicalSwitchName(LS3))
	if err != nil {
		assert.Errorf(t, err, "Cannot update lswitch with empty ext_id")
		t.Logf("Adding empty external_id for LS3 validation is ok")
	}
	err = ovndbapi.Execute(cmd)
	if err != nil {
		t.Fatal(err)
	}

}

func TestLogicalSwitchDelExternalIDs(t *testing.T) {
	t.Logf("DelExternalIDs LogicalSwitch")
	//delete external_id from LS3
	cmd, err := ovndbapi.LogicalSwitch.DelExternalIDs(
		LogicalSwitchName(LS3),
		LogicalSwitchExternalIDs(map[string]string{"neutron:network": "dummy"}))
	if err != nil {
		t.Fatal(err)
	}
	err = ovndbapi.Execute(cmd)
	if err != nil {
		t.Fatal(err)
	}
	// Get LS3 and get external_id
	ls, err := ovndbapi.LogicalSwitch.Get(LogicalSwitchName(LS3))
	if err != nil {
		t.Fatal(err)
	}
	for key, val := range ls.ExternalIDs {
		if key == FOO {
			assert.Equal(t, true, val == BAR, "Externel id with value dummy deleted")
			t.Logf("Deleted external_id key NEUTRON_NETWORK from LS3")
		}
	}
	// Delete empty external_ids from LS3
	cmd, err = ovndbapi.LogicalSwitch.DelExternalIDs(LogicalSwitchName(LS3))
	if err != nil {
		assert.Errorf(t, err, "Cannot update lswitch with empty ext_id")
		t.Logf("Deleting empty external_id from LS3 validation is ok")
	}
	err = ovndbapi.Execute(cmd)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLogicalSwitchDel(t *testing.T) {
	//t.Skip()
	t.Logf("Del LogicalSwitch")
	cmd, err := ovndbapi.LogicalSwitch.Del(LogicalSwitchName(LS3))
	if err != nil {
		t.Fatal(err)
	}
	err = ovndbapi.Execute(cmd)
	if err != nil {
		t.Fatal(err)
	}
	lsList, err := ovndbapi.LogicalSwitch.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(lsList) != 0 {
		t.Fatalf("invalid ls count found %#v\n", lsList)
	}
}
