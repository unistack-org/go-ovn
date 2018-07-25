/**
 * Copyright (c) 2017 eBay Inc.
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

package ovn

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"time"

	//	"github.com/oklog/ulid"
	"github.com/socketplane/libovsdb"
)

var (
	entropy *rand.Rand
)

var (
	ErrorExists = errors.New("Item already exists")
)

func init() {
	entropy = rand.New(rand.NewSource(time.Now().UnixNano()))
	//rand.Seed(MAX_TRANSACTION)
}

type OVNRow map[string]interface{}

func newNBImp(client *ovnDBClient, callback OVNSignal) (*ovnDBImp, error) {
	nbimp := ovnDBImp{client: client}
	nbimp.cache = make(map[string]map[string]libovsdb.Row)
	initial, err := nbimp.client.dbclient.MonitorAll(NBDB, "")
	if err != nil {
		return nil, err
	}
	nbimp.populateCache(*initial)
	notifier := ovnNotifier{&nbimp}
	nbimp.client.dbclient.Register(notifier)
	nbimp.callback = callback
	return &nbimp, nil
}

func (odbi *ovnDBImp) lswListImp() (*OvnCommand, error) {
	condition := libovsdb.NewCondition("name", "!=", "")
	listOp := libovsdb.Operation{
		Op:    OpList,
		Table: TableLS,
		Where: []interface{}{condition},
	}

	operations := []libovsdb.Operation{listOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) lswAddImp(lsw string) (*OvnCommand, error) {
	var err error
	namedUUID := "lsw_add" + strconv.Itoa(rand.Int())
	//namedUUID, err := ulid.New(ulid.Now(), entropy)
	if err != nil {
		return nil, err
	}
	//row to insert
	lswitch := make(OVNRow)
	lswitch["name"] = lsw

	if odbi.getRowUUID(TableLS, lswitch) != "" {
		return nil, ErrorExists
	}
	insertOp := libovsdb.Operation{
		Op:       OpInsert,
		Table:    TableLS,
		Row:      lswitch,
		UUIDName: namedUUID,
	}
	operations := []libovsdb.Operation{insertOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) lswDelImp(lsw string) (*OvnCommand, error) {
	condition := libovsdb.NewCondition("name", "==", lsw)
	delOp := libovsdb.Operation{
		Op:    OpDelete,
		Table: TableLS,
		Where: []interface{}{condition},
	}
	operations := []libovsdb.Operation{delOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) getRowUUID(table string, row OVNRow) string {
	odbi.cachemutex.Lock()
	defer odbi.cachemutex.Unlock()
	for uuid, drows := range odbi.cache[table] {
		found := false
		for field, value := range row {
			if v, ok := drows.Fields[field]; ok {
				if v == value {
					found = true
				} else {
					found = false
					break
				}
			}
		}
		if found {
			return uuid
		}
	}
	return ""
}

//test if map s contains t
//This function is not both s and t are nil at same time
func (odbi *ovnDBImp) oMapContians(s, t map[interface{}]interface{}) bool {
	if s == nil || t == nil {
		return false
	}

	for tk, tv := range t {
		if sv, ok := s[tk]; !ok {
			return false
		} else if tv != sv {
			return false
		}
	}
	return true
}

func (odbi *ovnDBImp) getACLUUIDByRow(lsw, table string, row OVNRow) string {
	odbi.cachemutex.Lock()
	defer odbi.cachemutex.Unlock()
	for _, drows := range odbi.cache[TableLS] {
		if rlsw, ok := drows.Fields["name"].(string); ok && rlsw == lsw {
			acls := drows.Fields["acls"]
			if acls != nil {
				switch acls.(type) {
				case libovsdb.OvsSet:
					if as, ok := acls.(libovsdb.OvsSet); ok {
						for _, a := range as.GoSet {
							if va, ok := a.(libovsdb.UUID); ok {
								for field, value := range row {
									switch field {
									case "action":
										if odbi.cache[TableACL][va.GoUUID].Fields["action"].(string) != value {
											goto unmatched
										}
									case "direction":
										if odbi.cache[TableACL][va.GoUUID].Fields["direction"].(string) != value {
											goto unmatched
										}
									case "match":
										if odbi.cache[TableACL][va.GoUUID].Fields["match"].(string) != value {
											goto unmatched
										}
									case "priority":
										if odbi.cache[TableACL][va.GoUUID].Fields["priority"].(int) != value {
											goto unmatched
										}
									case "log":
										if odbi.cache[TableACL][va.GoUUID].Fields["log"].(bool) != value {
											goto unmatched
										}
									case "external_ids":
										if value != nil && !odbi.oMapContians(odbi.cache[TableACL][va.GoUUID].Fields["external_ids"].(libovsdb.OvsMap).GoMap, value.(*libovsdb.OvsMap).GoMap) {
											goto unmatched
										}
									}
								}
								return va.GoUUID
							}
						unmatched:
						}
						return ""
					}
				case libovsdb.UUID:
					if va, ok := acls.(libovsdb.UUID); ok {
						for field, value := range row {
							switch field {
							case "action":
								if odbi.cache[TableACL][va.GoUUID].Fields["action"].(string) != value {
									goto out
								}
							case "direction":
								if odbi.cache[TableACL][va.GoUUID].Fields["direction"].(string) != value {
									goto out
								}
							case "match":
								if odbi.cache[TableACL][va.GoUUID].Fields["match"].(string) != value {
									goto out
								}
							case "priority":
								if odbi.cache[TableACL][va.GoUUID].Fields["priority"].(int) != value {
									goto out
								}
							case "log":
								if odbi.cache[TableACL][va.GoUUID].Fields["log"].(bool) != value {
									goto out
								}
							case "external_ids":
								if value != nil && !odbi.oMapContians(odbi.cache[TableACL][va.GoUUID].Fields["external_ids"].(libovsdb.OvsMap).GoMap, value.(*libovsdb.OvsMap).GoMap) {
									goto out
								}
							}
						}
						return va.GoUUID
					out:
					}
				}
			}
		}
	}
	return ""
}

func (odbi *ovnDBImp) getRowUUIDContainsUUID(table, field, uuid string) string {
	odbi.cachemutex.Lock()
	defer odbi.cachemutex.Unlock()
	for id, drows := range odbi.cache[table] {
		v := fmt.Sprintf("%s", drows.Fields[field])
		if strings.Contains(v, uuid) {
			return id
		}
	}
	return ""
}

func (odbi *ovnDBImp) lspAddImp(lsw, lsp string) (*OvnCommand, error) {
	var err error
	namedUUID := "lsp_add" + strconv.Itoa(rand.Int())
	//namedUUID, err := ulid.New(ulid.Now(), entropy)
	if err != nil {
		return nil, err
	}
	lsprow := make(OVNRow)
	lsprow["name"] = lsp

	if odbi.getRowUUID(TableLSP, lsprow) != "" {
		return nil, ErrorExists
	}

	insertOp := libovsdb.Operation{
		Op:       OpInsert,
		Table:    TableLSP,
		Row:      lsprow,
		UUIDName: namedUUID,
	}

	mutateUUID := []libovsdb.UUID{{namedUUID}}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
	mutation := libovsdb.NewMutation("ports", OpInsert, mutateSet)
	condition := libovsdb.NewCondition("name", "==", lsw)

	mutateOp := libovsdb.Operation{
		Op:        OpMutate,
		Table:     TableLS,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}
	operations := []libovsdb.Operation{insertOp, mutateOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) lspDelImp(lsp string) (*OvnCommand, error) {
	lsprow := make(OVNRow)
	lsprow["name"] = lsp
	lspUUID := odbi.getRowUUID(TableLSP, lsprow)
	mutateUUID := []libovsdb.UUID{{lspUUID}}
	condition := libovsdb.NewCondition("name", "==", lsp)
	delOp := libovsdb.Operation{
		Op:    OpDelete,
		Table: TableLSP,
		Where: []interface{}{condition},
	}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
	mutation := libovsdb.NewMutation("ports", OpDelete, mutateSet)
	mucondition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{odbi.getRowUUIDContainsUUID(TableLS, "ports", lspUUID)})
	// simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        OpMutate,
		Table:     TableLS,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{mucondition},
	}
	operations := []libovsdb.Operation{delOp, mutateOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) lspSetDHCPv4OptionsImp(lsp string, cidr string, opts map[string]string, external_ids map[string]string) (*OvnCommand, error) {
	var err error
	namedUUID := "dhcp_add" + strconv.Itoa(rand.Int())
	//namedUUID, err := ulid.New(ulid.Now(), entropy)
	if err != nil {
		return nil, err
	}
	dhcprow := make(OVNRow)
	dhcprow["cidr"] = cidr
	oOpts, err := libovsdb.NewOvsMap(opts)
	if err != nil {
		return nil, errors.New("Add DHCP: Opts is not correct in DHCP_Options")
	}
	dhcprow["options"] = oOpts

	if external_ids != nil {
		oMap, err := libovsdb.NewOvsMap(external_ids)
		if err != nil {
			return nil, errors.New("Add DHCP: External id is not correct in DHCP_Options")
		}
		dhcprow["external_ids"] = oMap
	}

	/*
		if odbi.getACLUUIDByRow(lsp, TableACL, aclrow) != "" {
			glog.V(OVNLOGLEVEL).Info("The acl existed, and will get nil command")
			return nil
		}
	*/

	insertOp := libovsdb.Operation{
		Op:       OpInsert,
		Table:    TableDHCPOptions,
		Row:      dhcprow,
		UUIDName: namedUUID,
	}

	mutateUUID := []libovsdb.UUID{{namedUUID}}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
	mutation := libovsdb.NewMutation("dhcpv4_options", OpInsert, mutateSet)
	condition := libovsdb.NewCondition("name", "==", lsp)

	// simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        OpMutate,
		Table:     TableLSP,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}
	operations := []libovsdb.Operation{insertOp, mutateOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) lspGetDHCPv4OptionsImp(lsp string) *OvnCommand {
	return nil
}

func (odbi *ovnDBImp) lspSetDHCPv6OptionsImp(lsp string, addr ...string) *OvnCommand {
	return nil
}

func (odbi *ovnDBImp) lspGetDHCPv6OptionsImp(lsp string) *OvnCommand {
	return nil
}

func (odbi *ovnDBImp) lspSetAddressImp(lsp string, addr ...string) (*OvnCommand, error) {
	row := make(OVNRow)
	addresses, _ := libovsdb.NewOvsSet(addr)
	row["addresses"] = addresses
	condition := libovsdb.NewCondition("name", "==", lsp)
	Op := libovsdb.Operation{
		Op:    OpUpdate,
		Table: TableLSP,
		Row:   row,
		Where: []interface{}{condition},
	}
	operations := []libovsdb.Operation{Op}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) lspSetPortSecurityImp(lsp string, security ...string) (*OvnCommand, error) {
	row := make(OVNRow)
	port_security, _ := libovsdb.NewOvsSet(security)
	row["port_security"] = port_security
	condition := libovsdb.NewCondition("name", "==", lsp)
	Op := libovsdb.Operation{
		Op:    OpUpdate,
		Table: TableLSP,
		Row:   row,
		Where: []interface{}{condition},
	}
	operations := []libovsdb.Operation{Op}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) aclAddImp(lsw, direct, match, action string, priority int, external_ids map[string]string, logflag bool) (*OvnCommand, error) {
	var err error
	namedUUID := "acl_add" + strconv.Itoa(rand.Int())
	//namedUUID, err := ulid.New(ulid.Now(), entropy)
	if err != nil {
		return nil, err
	}
	aclrow := make(OVNRow)
	aclrow["direction"] = direct
	aclrow["match"] = match
	aclrow["priority"] = priority

	if external_ids != nil {
		oMap, err := libovsdb.NewOvsMap(external_ids)
		if err != nil {
			return nil, errors.New("Add ACL: External id is not correct in acl")
		}
		aclrow["external_ids"] = oMap
	}

	if odbi.getACLUUIDByRow(lsw, TableACL, aclrow) != "" {
		return nil, ErrorExists
	}
	aclrow["action"] = action
	aclrow["log"] = logflag
	insertOp := libovsdb.Operation{
		Op:       OpInsert,
		Table:    TableACL,
		Row:      aclrow,
		UUIDName: namedUUID,
	}

	mutateUUID := []libovsdb.UUID{{namedUUID}}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
	mutation := libovsdb.NewMutation("acls", OpInsert, mutateSet)
	condition := libovsdb.NewCondition("name", "==", lsw)

	// simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        OpMutate,
		Table:     TableLS,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}
	operations := []libovsdb.Operation{insertOp, mutateOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) aclDelImp(lsw, direct, match string, priority int, external_ids map[string]string) (*OvnCommand, error) {
	aclrow := make(OVNRow)

	wherecondition := []interface{}{}
	if direct != "" {
		aclrow["direction"] = direct
	}
	if match != "" {
		aclrow["match"] = match
	}
	//in ovn pirority is greater than/equal 0,
	//if input the pirority < 0, lots of acls will be deleted if matches direct and match condition judgement.
	if priority >= 0 {
		aclrow["priority"] = priority
	}

	if external_ids != nil {
		oMap, err := libovsdb.NewOvsMap(external_ids)
		if err != nil {
			return nil, errors.New("Add ACL: External id is not correct in acl")
		}
		aclrow["external_ids"] = oMap
	}

	aclUUID := odbi.getACLUUIDByRow(lsw, TableACL, aclrow)
	if aclUUID == "" {
		return nil, errors.New("The deleting acl not found in cache, and will get nil command")
	}

	uuidcondition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{aclUUID})
	wherecondition = append(wherecondition, uuidcondition)
	delOp := libovsdb.Operation{
		Op:    OpDelete,
		Table: TableACL,
		Where: wherecondition,
	}

	mutation := libovsdb.NewMutation("acls", OpDelete, libovsdb.UUID{aclUUID})
	condition := libovsdb.NewCondition("name", "==", lsw)

	// Simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        OpMutate,
		Table:     TableLS,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}
	operations := []libovsdb.Operation{mutateOp, delOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) ASUpdate(name string, addrs []string, external_ids map[string]string) (*OvnCommand, error) {
	asrow := make(OVNRow)
	asrow["name"] = name
	addresses, _ := libovsdb.NewOvsSet(addrs)
	asrow["addresses"] = addresses
	if external_ids != nil {
		oMap, err := libovsdb.NewOvsMap(external_ids)
		if err != nil {
			return nil, errors.New("Add AS: External id is not correct in address set")
		}
		asrow["external_ids"] = oMap
	}
	condition := libovsdb.NewCondition("name", "==", name)
	Op := libovsdb.Operation{
		Op:    OpUpdate,
		Table: TableAS,
		Row:   asrow,
		Where: []interface{}{condition},
	}
	operations := []libovsdb.Operation{Op}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) ASAdd(name string, addrs []string, external_ids map[string]string) (*OvnCommand, error) {
	asrow := make(OVNRow)
	asrow["name"] = name
	//should support the -is-exist flag here.
	if odbi.getRowUUID(TableAS, asrow) != "" {
		return nil, errors.New("Add AS: already exists, get nil command")
	}
	if external_ids != nil {
		oMap, err := libovsdb.NewOvsMap(external_ids)
		if err != nil {
			return nil, errors.New("Add AS: External id is not correct in address set")
		}
		asrow["external_ids"] = oMap
	}
	addresses, _ := libovsdb.NewOvsSet(addrs)
	asrow["addresses"] = addresses
	Op := libovsdb.Operation{
		Op:    OpInsert,
		Table: TableAS,
		Row:   asrow,
	}
	operations := []libovsdb.Operation{Op}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) GetASByName(name string) *AddressSet {
	addresssets := odbi.GetAddressSets()
	for _, s := range addresssets {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func (odbi *ovnDBImp) ASDel(name string) (*OvnCommand, error) {
	condition := libovsdb.NewCondition("name", "==", name)
	delOp := libovsdb.Operation{
		Op:    OpDelete,
		Table: TableAS,
		Where: []interface{}{condition},
	}
	operations := []libovsdb.Operation{delOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) LSSetOpt(lsp string, options map[string]string) (*OvnCommand, error) {
	mutatemap, _ := libovsdb.NewOvsMap(options)
	mutation := libovsdb.NewMutation("options", OpInsert, mutatemap)
	condition := libovsdb.NewCondition("name", "==", lsp)

	// simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        OpMutate,
		Table:     TableLSP,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}
	operations := []libovsdb.Operation{mutateOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) transact(ops ...libovsdb.Operation) ([]libovsdb.OperationResult, error) {
	// Only support one trans at same time now.
	odbi.tranmutex.Lock()
	defer odbi.tranmutex.Unlock()
	reply, err := odbi.client.dbclient.Transact(NBDB, ops...)

	if err != nil {
		return reply, err
	}

	if len(reply) < len(ops) {
		//		glog.V(OVNLOGLEVEL).Info("Number of Replies should be atleast equal to number of operations")
		for i, o := range reply {
			if o.Error != "" && i < len(ops) {
				//	glog.V(OVNLOGLEVEL).Info("Transaction Failed due to an error :", o.Error, " details:", o.Details, " in ", ops[i])
				return nil, errors.New(fmt.Sprint("Transaction Failed due to an error :", o.Error, " details:", o.Details, " in ", ops[i]))
			}
		}
		return reply, errors.New(fmt.Sprint("Number of Replies should be atleast equal to number of operations"))
	}
	//glog.V(OVNLOGLEVEL).Info("transaction reply : ", reply)
	return reply, nil
}

func (odbi *ovnDBImp) Execute(cmds ...*OvnCommand) error {
	if cmds == nil {
		return nil
	}
	var ops []libovsdb.Operation
	for _, cmd := range cmds {
		if cmd != nil {
			ops = append(ops, cmd.Operations...)
		}
	}
	_, err := odbi.transact(ops...)
	//	glog.V(OVNLOGLEVEL).Infof("OVN replys: %v", reply)
	if err != nil {
		return err
	}
	return nil
}

func (odbi *ovnDBImp) float64_to_int(row libovsdb.Row) {
	for field, value := range row.Fields {
		if v, ok := value.(float64); ok {
			n := int(v)
			if float64(n) == v {
				row.Fields[field] = n
			}
		}
	}
}

func (odbi *ovnDBImp) populateCache(updates libovsdb.TableUpdates) {
	//glog.V(OVNLOGLEVEL).Info("New nofity arrived")
	odbi.cachemutex.Lock()
	defer odbi.cachemutex.Unlock()
	for table, tableUpdate := range updates.Updates {
		if _, ok := odbi.cache[table]; !ok {
			odbi.cache[table] = make(map[string]libovsdb.Row)
		}
		for uuid, row := range tableUpdate.Rows {
			// TODO: this is a workaround for the problem of
			// missing json number conversion in libovsdb
			odbi.float64_to_int(row.New)

			empty := libovsdb.Row{}
			if !reflect.DeepEqual(row.New, empty) {
				odbi.cache[table][uuid] = row.New
				if odbi.callback != nil {
					switch table {
					case TableLSP:
						lp := odbi.RowToLogicalPort(uuid)
						odbi.callback.OnLogicalPortCreate(lp)
					case TableACL:
						acl := odbi.RowToACL(uuid)
						odbi.callback.OnACLCreate(acl)
					}
				}
			} else {
				if odbi.callback != nil {
					switch table {
					case TableLSP:
						lp := odbi.RowToLogicalPort(uuid)
						odbi.callback.OnLogicalPortDelete(lp)
					case TableACL:
						acl := odbi.RowToACL(uuid)
						odbi.callback.OnACLDelete(acl)
					}
				}
				delete(odbi.cache[table], uuid)
			}
		}
	}
}

func (odbi *ovnDBImp) ConvertGoSetToStringArray(oset libovsdb.OvsSet) []string {
	var ret = []string{}
	for _, s := range oset.GoSet {
		value, ok := s.(string)
		if ok {
			ret = append(ret, value)
		}
	}
	return ret
}

func (odbi *ovnDBImp) RowToLogicalPort(uuid string) *LogcalPort {
	lp := &LogcalPort{
		UUID: uuid,
		Name: odbi.cache[TableLSP][uuid].Fields["name"].(string),
	}
	addr := odbi.cache[TableLSP][uuid].Fields["addresses"]
	switch addr.(type) {
	case string:
		lp.Addresses = []string{addr.(string)}
	case libovsdb.OvsSet:
		lp.Addresses = odbi.ConvertGoSetToStringArray(addr.(libovsdb.OvsSet))
	default:
		//glog.V(OVNLOGLEVEL).Info("Unsupport type found in lport address.")
	}
	portsecurity := odbi.cache[TableLSP][uuid].Fields["port_security"]
	switch portsecurity.(type) {
	case string:
		lp.PortSecurity = []string{portsecurity.(string)}
	case libovsdb.OvsSet:
		lp.PortSecurity = odbi.ConvertGoSetToStringArray(portsecurity.(libovsdb.OvsSet))
	default:
		//glog.V(OVNLOGLEVEL).Info("Unsupport type found in lport port security.")
	}
	return lp
}

// Get all lport by lswitch
func (odbi *ovnDBImp) GetLogicPortsBySwitch(lsw string) []*LogcalPort {
	var lplist = []*LogcalPort{}
	odbi.cachemutex.Lock()
	defer odbi.cachemutex.Unlock()
	for _, drows := range odbi.cache[TableLS] {
		if rlsw, ok := drows.Fields["name"].(string); ok && rlsw == lsw {
			ports := drows.Fields["ports"]
			if ports != nil {
				switch ports.(type) {
				case libovsdb.OvsSet:
					if ps, ok := ports.(libovsdb.OvsSet); ok {
						for _, p := range ps.GoSet {
							if vp, ok := p.(libovsdb.UUID); ok {
								tp := odbi.RowToLogicalPort(vp.GoUUID)
								lplist = append(lplist, tp)
							}
						}
					} else {
						//			glog.V(OVNLOGLEVEL).Info("Type libovsdb.OvsSet casting failed.")
					}
				case libovsdb.UUID:
					if vp, ok := ports.(libovsdb.UUID); ok {
						tp := odbi.RowToLogicalPort(vp.GoUUID)
						lplist = append(lplist, tp)
					} else {
						//		glog.V(OVNLOGLEVEL).Info("Type libovsdb.UUID casting failed.")
					}
				default:
					//	glog.V(OVNLOGLEVEL).Info("Unsupport type found in ovsdb rows.")
				}
			}
			break
		}
	}
	return lplist
}

func (odbi *ovnDBImp) RowToACL(uuid string) *ACL {
	acl := &ACL{
		UUID:       uuid,
		Action:     odbi.cache[TableACL][uuid].Fields["action"].(string),
		Direction:  odbi.cache[TableACL][uuid].Fields["direction"].(string),
		Match:      odbi.cache[TableACL][uuid].Fields["match"].(string),
		Priority:   odbi.cache[TableACL][uuid].Fields["priority"].(int),
		Log:        odbi.cache[TableACL][uuid].Fields["log"].(bool),
		ExternalID: odbi.cache[TableACL][uuid].Fields["external_ids"].(libovsdb.OvsMap).GoMap,
	}
	return acl
}

// Get all acl by lswitch
func (odbi *ovnDBImp) GetACLsBySwitch(lsw string) []*ACL {
	//TODO: should be improvement here, when have lots of acls.
	acllist := make([]*ACL, 0, 0)
	odbi.cachemutex.Lock()
	defer odbi.cachemutex.Unlock()
	for _, drows := range odbi.cache[TableLS] {
		if rlsw, ok := drows.Fields["name"].(string); ok && rlsw == lsw {
			acls := drows.Fields["acls"]
			if acls != nil {
				switch acls.(type) {
				case libovsdb.OvsSet:
					if as, ok := acls.(libovsdb.OvsSet); ok {
						for _, a := range as.GoSet {
							if va, ok := a.(libovsdb.UUID); ok {
								ta := odbi.RowToACL(va.GoUUID)
								acllist = append(acllist, ta)
							}
						}
					}
				case libovsdb.UUID:
					if va, ok := acls.(libovsdb.UUID); ok {
						ta := odbi.RowToACL(va.GoUUID)
						acllist = append(acllist, ta)
					}
				}
			}
			break
		}
	}
	return acllist
}

// Get all addressset
func (odbi *ovnDBImp) GetAddressSets() []*AddressSet {
	adlist := make([]*AddressSet, 0, 0)
	odbi.cachemutex.Lock()
	defer odbi.cachemutex.Unlock()
	for uuid, drows := range odbi.cache[TableAS] {
		ta := &AddressSet{
			UUID:       uuid,
			Name:       drows.Fields["name"].(string),
			ExternalID: drows.Fields["external_ids"].(libovsdb.OvsMap).GoMap,
		}
		addresses := []string{}
		as := drows.Fields["addresses"]
		switch as.(type) {
		case libovsdb.OvsSet:
			//TODO: is it possible return interface type directly instead of GoSet
			if goset, ok := drows.Fields["addresses"].(libovsdb.OvsSet); ok {
				for _, i := range goset.GoSet {
					addresses = append(addresses, i.(string))
				}
			}
		case string:
			if v, ok := drows.Fields["addresses"].(string); ok {
				addresses = append(addresses, v)
			}
		}
		ta.Addresses = addresses
		adlist = append(adlist, ta)
	}
	return adlist
}