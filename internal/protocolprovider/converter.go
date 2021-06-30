package protocol

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type Converter struct {
	Schema  *tfprotov5.SchemaBlock
	AsGo    func(tftypes.Value, interface{}) error
	AsValue func(interface{}) tftypes.Value
}

func NewConverter(val reflect.Type) (*Converter, error) {
	// We look at the field tags to find how to convert the Go type to a
	// similar Terraform schema, and build functions that will convert the
	// tftype.Value to a Go struct and vice-versa.

	// First, let's find the type of the tags in this struct. HCLv2 accepts
	// 'optional', 'block', 'label', 'attr', and 'remain' but Nomad only uses
	// the first three so we will only support that. By focusing only on what
	// Nomad needs we can go straight to the point, only do the bare minimum
	// and handle edge cases explicitely when there is some (for example for
	// the config and policy stanza) rather than making some general converter
	// which would be a much more difficult endeavor.

	labels, err := getTaggedFields("label", val)
	if err != nil {
		return nil, err
	}

	// It is possible for a block to have multiple labels in HCLv2 but there
	// is none like that in Nomad for now so we return an error immediately
	// if this hypothesis does not hold, this way the tests will fail as
	// soon as we update the Nomad api client and we will be able to update
	// this function based on the new behavior.
	if len(labels) > 1 {
		return nil, fmt.Errorf("multiple labels found")
	} else if len(labels) == 1 {

		var field reflect.StructField
		var name string
		for n, f := range labels {
			field = f
			name = n
		}

		if t := field.Type.String(); t != "string" && t != "*string" {
			// The label of a block should always be a string
			return nil, fmt.Errorf("wrong type %q for label %q", t, name)
		}
	}

	fields := Fields{}

	// Let's now add all attributes to the schema and create their converters
	optionals, err := getTaggedFields("optional", val)
	if err != nil {
		return nil, err
	}

	for name, field := range optionals {
		// The interesting part for the attributes is handled by the converters.
		// A very important thing of using terraform-plugin-go is that we can
		// detect unset attributes (they would default to the default value in
		// terraform-plugin-sdk) so that we can leave a nil pointer and the
		// server will choose the default instead of us having to guess. This is
		// great because the default may change between versions, it is
		// error-prone to copy them from the docs or Nomad code source and they
		// may depend on external factor that the provider may not know.
		attr, err := NewAttribute(field.Type, field.Name, name)
		if err != nil {
			return nil, err
		}
		fields = append(fields, attr)
	}

	// The last thing to do is to convert the blocks
	blocks, err := getTaggedFields("block", val)
	if err != nil {
		return nil, err
	}

	for name, field := range blocks {

		// If the block is a pointer to a struct we need to convert the actual
		// struct
		val := field.Type
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}

		if val.Kind() == reflect.Map {
			// Maps are not blocks in Terraform and must be an attribute in
			// protocol 5. Since terraform-plugin-mux does not support
			// protocol 6 we use an attribute here, which means the
			// Terraform configuration will have a = sign where the Nomad
			// file does not need it, but this is a small disagreement
			// that we should be able to fix later.
			attr, err := NewAttribute(val, field.Name, name)
			if err != nil {
				return nil, err
			}
			fields = append(fields, attr)

		} else {
			// Now that we handled the labels, the attributes and the map blocks
			// that needed to be converted as attributes for the moment we can
			// finally convert the remaining blocks.
			// There is three types of block that we need to concern ourselves with:
			//   - a Single block, which is a block that can appear at most once in
			//     the configuration
			//   - a List block, which may appear multiple time
			//   - a Map block, which is a block with a label
			// While we can see the difference between the first two just by looking
			// at the current struct, finding out whether it should be a list or a
			// map requires to look inside the child to see if it has a label.
			block, err := NewBlock(val, field.Name, name)
			if err != nil {
				return nil, err
			}
			fields = append(fields, block)
		}
	}

	return &Converter{
		Schema: &tfprotov5.SchemaBlock{
			Version:    1,
			Attributes: fields.GetAttributesSchema(),
			BlockTypes: fields.GetBlocksSchema(),
		},
		AsGo: func(val tftypes.Value, obj interface{}) error {
			o := reflect.Indirect(reflect.ValueOf(obj))

			for _, field := range fields {
				f := o.FieldByName(field.GetGoName())
				if err := field.AsGo(val, f); err != nil {
					return err
				}
			}

			return nil
		},
		AsValue: func(obj interface{}) tftypes.Value {
			val := map[string]tftypes.Value{}
			attrs := map[string]tftypes.Type{}
			optionalAttrs := map[string]struct{}{}
			o := reflect.Indirect(reflect.ValueOf(obj))

			for _, field := range fields {

				var e interface{}
				if o.IsValid() {
					f := o.FieldByName(field.GetGoName())
					e = f.Interface()
				}

				v := field.AsValue(e)
				val[field.GetValueName()] = v
				attrs[field.GetValueName()] = v.Type()
			}

			return tftypes.NewValue(
				tftypes.Object{
					AttributeTypes:     attrs,
					OptionalAttributes: optionalAttrs,
				},
				val,
			)
		},
	}, nil
}

func getTaggedFields(kind string, val reflect.Type) (map[string]reflect.StructField, error) {
	fields := map[string]reflect.StructField{}

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		tag, ok := field.Tag.Lookup("hcl")

		if !ok {
			// If the field does not have an hcl tag we just skip it.
			continue
		}

		parts := strings.SplitN(tag, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("wrong number of parts for %q", tag)
		}

		name := parts[0]

		if kind == parts[1] {
			fields[name] = field
		}
	}

	return fields, nil
}

func getLabelFieldName(val reflect.Type) (string, error) {
	var field reflect.StructField
	labels, err := getTaggedFields("label", val)
	if err != nil {
		return "", err
	}

	// It is possible for a block to have multiple labels in HCLv2 but there
	// is none like that in Nomad for now so we return an error immediately
	// if this hypothesis does not hold, this way the tests will fail as
	// soon as we update the Nomad api client and we will be able to update
	// this function based on the new behavior.
	if len(labels) > 1 {
		return "", fmt.Errorf("multiple labels found")
	} else if len(labels) == 0 {
		return "", fmt.Errorf("no label found in %q", val.String())
	} else if len(labels) == 1 {

		for _, f := range labels {
			field = f
		}

		if t := field.Type.String(); t != "string" && t != "*string" {
			// The label of a block should always be a string
			return "", fmt.Errorf("wrong type %q for label %q", t, field.Name)
		}
	}

	return field.Name, nil
}

type Field interface {
	GetGoName() string
	GetValueName() string
	AsGo(tftypes.Value, reflect.Value) error
	AsValue(interface{}) tftypes.Value
}

type Fields []Field

type Attributes []*tfprotov5.SchemaAttribute

func (a Attributes) Len() int           { return len(a) }
func (a Attributes) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Attributes) Less(i, j int) bool { return a[i].Name < a[j].Name }

func (f Fields) GetAttributesSchema() (attrs []*tfprotov5.SchemaAttribute) {
	for _, field := range f {
		if attr, ok := field.(*Attribute); ok {
			attrs = append(attrs, attr.GetSchema())
		}
	}

	// To make it easier to test we sort the attributes so the result will
	// be deterministic
	sort.Sort(Attributes(attrs))

	return
}

func (f Fields) GetBlocksSchema() (blocks []*tfprotov5.SchemaNestedBlock) {
	for _, field := range f {
		if block, ok := field.(*Block); ok {
			blocks = append(blocks, block.GetSchema())
		}
	}

	return
}

type Attribute struct {
	goName    string
	valueName string
	asGo      func(tftypes.Value, reflect.Value) error
	asValue   func(interface{}) tftypes.Value
	_type     tftypes.Type
}

func (a Attribute) AsGo(val tftypes.Value, obj reflect.Value) error {
	var v map[string]tftypes.Value
	if err := val.As(&v); err != nil {
		return err
	}
	return a.asGo(v[a.valueName], obj)
}

func (a Attribute) AsValue(obj interface{}) tftypes.Value {
	return a.asValue(obj)
}

func (a Attribute) GetGoName() string {
	return a.goName
}

func (a Attribute) GetValueName() string {
	return a.valueName
}

func (a Attribute) GetSchema() *tfprotov5.SchemaAttribute {
	return &tfprotov5.SchemaAttribute{
		Name: a.valueName,
		Type: a._type,

		// This may not be expected as many attributes are actually required
		// in a Nomad job definition but there are actually all set as
		// optional in the field tags. While we could go over each of them
		// manually to see whether or not they are required, it would be
		// difficult to get right and time consuming to do manually for
		// each new Nomad release. Worse it may evolve between two Nomad
		// versions and the provider cannot always expect to speak to an
		// up-to-date server so it may be detrimental to users to be overly
		// strict in the schema, it's better to try to create the job and
		// report a failure if something is missing than refusing to register
		// a job that is actually correct.
		Optional: true,
	}
}

type Block struct {
	goName    string
	valueName string
	asGo      func(tftypes.Value, reflect.Value) error
	asValue   func(interface{}) tftypes.Value
	mode      tfprotov5.SchemaNestedBlockNestingMode
	schema    *tfprotov5.SchemaBlock
}

func (b Block) AsGo(val tftypes.Value, obj reflect.Value) error {
	var v map[string]tftypes.Value
	if err := val.As(&v); err != nil {
		return err
	}
	return b.asGo(v[b.valueName], obj)
}

func (b Block) AsValue(obj interface{}) tftypes.Value {
	return b.asValue(obj)
}

func (b Block) GetGoName() string {
	return b.goName
}

func (b Block) GetValueName() string {
	return b.valueName
}

func (b Block) GetSchema() *tfprotov5.SchemaNestedBlock {
	return &tfprotov5.SchemaNestedBlock{
		TypeName: b.valueName,
		Nesting:  b.mode,
		Block:    b.schema,
	}
}

// Storing all attributes builders in this map will help make sure that we
// test them all in the tests
var attributeBuilder = map[string]func(string, string) *Attribute{
	"string":                        NewStringAttribute,
	"*string":                       NewStringAttribute,
	"time.Duration":                 NewStringAttribute,
	"*time.Duration":                NewStringAttribute,
	"api.CSIPluginType":             NewStringAttribute,
	"int":                           NewNumberAttribute,
	"*int":                          NewNumberAttribute,
	"*int8":                         NewNumberAttribute,
	"*uint64":                       NewNumberAttribute,
	"*int64":                        NewNumberAttribute,
	"uint8":                         NewNumberAttribute,
	"bool":                          NewBoolAttribute,
	"*bool":                         NewBoolAttribute,
	"[]string":                      NewStringListAttribute,
	"map[string]interface {}":       NewInterfaceMapAttribute,
	"map[string]string":             NewStringMapAttribute,
	"map[string][]string":           NewStringListMapAttribute,
	"map[string]*api.VolumeRequest": NewVolumeMapAttribute,
	"map[string]*api.ConsulGatewayBindAddress": NewConsulMapAttribute,
}

func NewAttribute(t reflect.Type, goName, valueName string) (*Attribute, error) {
	if builder, ok := attributeBuilder[t.String()]; ok {
		return builder(goName, valueName), nil
	}
	return nil, fmt.Errorf("unknown type %q", t.String())
}

func NewStringMapAttribute(goName, valueName string) *Attribute {
	return &Attribute{
		goName:    goName,
		valueName: valueName,
		_type: tftypes.Map{
			AttributeType: tftypes.String,
		},
		asGo: func(val tftypes.Value, obj reflect.Value) error {
			if val.IsNull() {
				return nil
			}

			var m map[string]tftypes.Value
			if err := val.As(&m); err != nil {
				return fmt.Errorf("failed to convert value %#v: %s", val, err)
			}

			res := map[string]string{}
			for name, v := range m {
				var s string
				if err := v.As(&s); err != nil {
					return fmt.Errorf("failed to convert value %#v: %s", v, err)
				}
				res[name] = s
			}

			obj.Set(reflect.ValueOf(res))
			return nil
		},
		asValue: func(obj interface{}) tftypes.Value {
			var m map[string]tftypes.Value

			if obj != nil {
				m = map[string]tftypes.Value{}

				for key, value := range obj.(map[string]string) {
					v := tftypes.NewValue(tftypes.String, value)
					m[key] = v
				}
			}

			return tftypes.NewValue(
				tftypes.Map{
					AttributeType: tftypes.String,
				},
				m,
			)
		},
	}
}

func NewStringAttribute(goName, valueName string) *Attribute {
	return &Attribute{
		goName:    goName,
		valueName: valueName,
		_type:     tftypes.String,
		asGo: func(val tftypes.Value, obj reflect.Value) error {
			if val.IsNull() {
				return nil
			}

			var s string
			if err := val.As(&s); err != nil {
				return fmt.Errorf("failed to convert value %#v: %s", val, err)
			}
			switch t := obj.Type().String(); t {
			case "string", "api.CSIPluginType":
				obj.SetString(s)
			case "*string":
				obj.Set(reflect.ValueOf(&s))
			case "time.Duration":
				d, err := time.ParseDuration(s)
				if err != nil {
					return fmt.Errorf("failed to parse %q: %s", s, err)
				}
				obj.Set(reflect.ValueOf(d))
			case "*time.Duration":
				if s == "" {
					return nil
				}
				d, err := time.ParseDuration(s)
				if err != nil {
					return fmt.Errorf("failed to parse %q: %s", s, err)
				}
				obj.Set(reflect.ValueOf(&d))
			default:
				return fmt.Errorf("unknown type %q", t)
			}

			return nil
		},
		asValue: func(obj interface{}) tftypes.Value {
			if obj == nil {
				return tftypes.NewValue(tftypes.String, nil)
			}

			switch o := obj.(type) {
			case time.Duration:
				return tftypes.NewValue(tftypes.String, o.String())
			case *time.Duration:
				if o == nil {
					return tftypes.NewValue(tftypes.String, "")
				}
				return tftypes.NewValue(tftypes.String, o.String())
			case api.CSIPluginType:
				return tftypes.NewValue(tftypes.String, string(o))
			default:
				return tftypes.NewValue(tftypes.String, o)
			}
		},
	}
}

func NewNumberAttribute(goName, valueName string) *Attribute {
	return &Attribute{
		goName:    goName,
		valueName: valueName,
		_type:     tftypes.Number,
		asGo: func(val tftypes.Value, obj reflect.Value) error {
			if val.IsNull() {
				return nil
			}
			var n big.Float
			if err := val.As(&n); err != nil {
				return fmt.Errorf("failed to convert value %#v: %s", val, err)
			}
			switch t := obj.Type().String(); t {
			case "int":
				i64, _ := n.Int64()
				obj.SetInt(i64)
			case "uint8":
				u64, _ := n.Uint64()
				obj.SetUint(u64)
			case "*uint64":
				i64, _ := n.Int64()
				i := uint64(i64)
				obj.Set(reflect.ValueOf(&i))
			case "*int8":
				i64, _ := n.Int64()
				i := int8(i64)
				obj.Set(reflect.ValueOf(&i))
			case "*int64":
				i64, _ := n.Int64()
				obj.Set(reflect.ValueOf(&i64))
			case "*int":
				i64, _ := n.Int64()
				i := int(i64)
				obj.Set(reflect.ValueOf(&i))
			default:
				return fmt.Errorf("unknown type %q", t)
			}

			return nil
		},
		asValue: func(obj interface{}) tftypes.Value {
			return tftypes.NewValue(tftypes.Number, obj)
		},
	}
}

func NewBoolAttribute(goName, valueName string) *Attribute {
	return &Attribute{
		goName:    goName,
		valueName: valueName,
		_type:     tftypes.Bool,
		asGo: func(val tftypes.Value, obj reflect.Value) error {
			if val.IsNull() {
				return nil
			}

			var b bool
			if err := val.As(&b); err != nil {
				return fmt.Errorf("failed to convert value %#v: %s", val, err)
			}
			switch t := obj.Type().String(); t {
			case "bool":
				obj.SetBool(b)
			case "*bool":
				obj.Set(reflect.ValueOf(&b))
			default:
				return fmt.Errorf("unknown type %q", t)
			}

			return nil
		},
		asValue: func(obj interface{}) tftypes.Value {
			return tftypes.NewValue(tftypes.Bool, obj)
		},
	}
}

func NewStringListAttribute(goName, valueName string) *Attribute {
	return &Attribute{
		goName:    goName,
		valueName: valueName,
		_type: tftypes.List{
			ElementType: tftypes.String,
		},
		asGo: func(val tftypes.Value, obj reflect.Value) error {
			if val.IsNull() {
				return nil
			}

			var values []tftypes.Value
			if err := val.As(&values); err != nil {
				return fmt.Errorf("failed to convert value %#v: %s", val, err)
			}

			l := []string{}
			for _, v := range values {
				var s string
				if err := v.As(&s); err != nil {
					return fmt.Errorf("failed to convert value %#v: %s", val, err)
				}
				l = append(l, s)
			}
			obj.Set(reflect.ValueOf(l))
			return nil
		},
		asValue: func(obj interface{}) tftypes.Value {
			ty := tftypes.List{
				ElementType: tftypes.String,
			}

			if obj == nil {
				return tftypes.NewValue(ty, nil)
			}

			val := []tftypes.Value{}
			for _, s := range obj.([]string) {
				val = append(val, tftypes.NewValue(tftypes.String, s))
			}
			return tftypes.NewValue(ty, val)
		},
	}
}

func NewInterfaceMapAttribute(goName, valueName string) *Attribute {
	// This is another attribute a bit weird: in a Nomad job spec,
	// both the driver 'config' stanza and the autoscaling 'policy'
	// are completely opaque to Nomad and given as is to the plugin.
	// It's a block, and may have any attributes of any type.
	// Looking at the code of tfprotov5 it seems to me that the
	// only thing that can have unknown keys is a map, but the value
	// all need to be the same type in tfprotov5 so we can't have
	// a schema for this:
	//   config {
	//     image = "hashicorp/http-echo"
	//     args  = ["-text", "hello"]
	//   }
	// Because of this we will write those fields as a JSON string
	// in the configuration and have a custom converter for them.
	// Looking at the code for tfprotov6, it looks like it would be
	// possible to lift these restrictions with it, but
	// terraform-plugin-mux does not support it yet so we will have
	// to improve this later

	return &Attribute{
		goName:    goName,
		valueName: valueName,
		_type:     tftypes.String,
		asGo: func(val tftypes.Value, obj reflect.Value) error {
			var s string
			if err := val.As(&s); err != nil {
				return err
			}

			var c map[string]interface{}
			if err := json.Unmarshal([]byte(s), &c); err != nil {
				return err
			}
			obj.Set(reflect.ValueOf(c))
			return nil
		},
		asValue: func(obj interface{}) tftypes.Value {
			s, _ := json.Marshal(obj)
			return tftypes.NewValue(tftypes.String, string(s))
		},
	}
}

func NewStringListMapAttribute(goName, valueName string) *Attribute {
	ty := tftypes.Map{
		AttributeType: tftypes.List{
			ElementType: tftypes.String,
		},
	}

	return &Attribute{
		goName:    goName,
		valueName: valueName,
		_type:     ty,
		asGo: func(val tftypes.Value, obj reflect.Value) error {
			c := map[string][]string{}
			var m map[string]tftypes.Value
			if err := val.As(&m); err != nil {
				return err
			}

			for key, value := range m {
				l := []string{}
				var values []tftypes.Value
				if err := value.As(&values); err != nil {
					return err
				}

				for _, v := range values {
					var s string
					if err := v.As(&s); err != nil {
						return err
					}
					l = append(l, s)
				}

				c[key] = l
			}

			obj.Set(reflect.ValueOf(c))
			return nil
		},
		asValue: func(obj interface{}) tftypes.Value {
			if obj == nil {
				return tftypes.NewValue(ty, nil)
			}

			m := map[string]tftypes.Value{}

			for key, value := range obj.(map[string][]string) {
				val := []tftypes.Value{}
				for _, v := range value {
					val = append(val, tftypes.NewValue(tftypes.String, v))
				}
				m[key] = tftypes.NewValue(ty.AttributeType, val)
			}

			return tftypes.NewValue(ty, m)
		},
	}
}

func NewVolumeMapAttribute(goName, valueName string) *Attribute {
	ty := tftypes.Map{
		AttributeType: tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"type":            tftypes.String,
				"source":          tftypes.String,
				"read_only":       tftypes.Bool,
				"access_mode":     tftypes.String,
				"attachment_mode": tftypes.String,
				"per_alloc":       tftypes.Bool,
			},
			OptionalAttributes: map[string]struct{}{
				"type":            {},
				"source":          {},
				"read_only":       {},
				"access_mode":     {},
				"attachment_mode": {},
				"per_alloc":       {},
			},
		},
	}
	return &Attribute{
		goName:    goName,
		valueName: valueName,
		_type:     ty,
		asGo: func(val tftypes.Value, obj reflect.Value) error {
			if val.IsNull() {
				return nil
			}

			var v map[string]tftypes.Value
			if err := val.As(&v); err != nil {
				return err
			}

			res := map[string]*api.VolumeRequest{}
			for key, item := range v {
				var i map[string]tftypes.Value
				if err := item.As(&i); err != nil {
					return err
				}

				var ty, source, access_mode, attachment_mode string
				if err := i["type"].As(&ty); err != nil {
					return err
				}
				if err := i["source"].As(&source); err != nil {
					return err
				}
				if err := i["access_mode"].As(&access_mode); err != nil {
					return err
				}
				if err := i["attachment_mode"].As(&attachment_mode); err != nil {
					return err
				}

				var read_only, per_alloc bool
				if err := i["read_only"].As(&read_only); err != nil {
					return err
				}
				if err := i["per_alloc"].As(&per_alloc); err != nil {
					return err
				}

				res[key] = &api.VolumeRequest{
					Name:           key,
					Type:           ty,
					Source:         source,
					AccessMode:     access_mode,
					AttachmentMode: attachment_mode,
					ReadOnly:       read_only,
					PerAlloc:       per_alloc,
				}
			}
			obj.Set(reflect.ValueOf(res))

			return nil
		},
		asValue: func(obj interface{}) tftypes.Value {
			if obj == nil {
				return tftypes.NewValue(ty, nil)
			}

			res := map[string]tftypes.Value{}
			for key, vol := range obj.(map[string]*api.VolumeRequest) {
				v := map[string]tftypes.Value{
					"type":            tftypes.NewValue(tftypes.String, vol.Type),
					"source":          tftypes.NewValue(tftypes.String, vol.Source),
					"read_only":       tftypes.NewValue(tftypes.Bool, vol.ReadOnly),
					"access_mode":     tftypes.NewValue(tftypes.String, vol.AccessMode),
					"attachment_mode": tftypes.NewValue(tftypes.String, vol.AttachmentMode),
					"per_alloc":       tftypes.NewValue(tftypes.Bool, vol.PerAlloc),
				}
				res[key] = tftypes.NewValue(ty.AttributeType, v)
			}

			return tftypes.NewValue(ty, res)
		},
	}
}

func NewConsulMapAttribute(goName, valueName string) *Attribute {
	ty := tftypes.Map{
		AttributeType: tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"address": tftypes.String,
				"port":    tftypes.Number,
			},
		},
	}
	return &Attribute{
		goName:    goName,
		valueName: valueName,
		_type:     ty,
		asGo: func(val tftypes.Value, obj reflect.Value) error {
			if val.IsNull() {
				return nil
			}

			var v map[string]tftypes.Value
			if err := val.As(&v); err != nil {
				return err
			}

			res := map[string]*api.ConsulGatewayBindAddress{}
			for key, item := range v {
				var i map[string]tftypes.Value
				if err := item.As(&i); err != nil {
					return err
				}

				var address string
				var port big.Float
				if err := i["address"].As(&address); err != nil {
					return err
				}
				if err := i["port"].As(&port); err != nil {
					return err
				}

				p, _ := port.Int64()

				res[key] = &api.ConsulGatewayBindAddress{
					Name:    key,
					Address: address,
					Port:    int(p),
				}
			}
			obj.Set(reflect.ValueOf(res))

			return nil
		},
		asValue: func(obj interface{}) tftypes.Value {
			if obj == nil {
				return tftypes.NewValue(ty, nil)
			}

			res := map[string]tftypes.Value{}
			for key, addr := range obj.(map[string]*api.ConsulGatewayBindAddress) {
				a := map[string]tftypes.Value{
					"address": tftypes.NewValue(tftypes.String, addr.Address),
					"port":    tftypes.NewValue(tftypes.Number, addr.Port),
				}
				res[key] = tftypes.NewValue(ty.AttributeType, a)
			}

			return tftypes.NewValue(ty, res)
		},
	}
}

func NewBlock(val reflect.Type, goName, valueName string) (*Block, error) {
	isSlice := val.Kind() == reflect.Slice

	if val.Kind() == reflect.Slice {
		val = val.Elem()
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
	}

	labels, err := getTaggedFields("label", val)
	if err != nil {
		return nil, err
	}

	hasLabels := len(labels) != 0

	switch {
	case hasLabels:
		return NewMapBlock(val, goName, valueName)
	case isSlice:
		return NewListBlock(val, goName, valueName)
	default:
		return NewSingleBlock(val, goName, valueName)
	}
}

func NewSingleBlock(val reflect.Type, goName, valueName string) (*Block, error) {
	c, err := NewConverter(val)
	if err != nil {
		return nil, err
	}

	return &Block{
		goName:    goName,
		valueName: valueName,
		mode:      tfprotov5.SchemaNestedBlockNestingModeSingle,
		schema:    c.Schema,
		asGo: func(val tftypes.Value, obj reflect.Value) error {
			if val.IsNull() {
				return nil
			}

			o := reflect.New(obj.Type().Elem()).Interface()
			if err := c.AsGo(val, o); err != nil {
				return err
			}
			obj.Set(reflect.ValueOf(o))
			return nil
		},
		asValue: func(obj interface{}) tftypes.Value {
			return c.AsValue(obj)
		},
	}, nil
}

func NewListBlock(val reflect.Type, goName, valueName string) (*Block, error) {
	c, err := NewConverter(val)
	if err != nil {
		return nil, err
	}

	return &Block{
		goName:    goName,
		valueName: valueName,
		mode:      tfprotov5.SchemaNestedBlockNestingModeList,
		schema:    c.Schema,
		asGo: func(val tftypes.Value, obj reflect.Value) error {
			if val.IsNull() {
				return nil
			}
			// If the block can appear multiple time, we
			// need to convert each of the sub-blocks, put them
			// in a slice and store that in the object.
			o := reflect.MakeSlice(obj.Type(), 0, 0)

			var v []tftypes.Value
			if err := val.As(&v); err != nil {
				return err
			}

			var conv func(val tftypes.Value) error
			if t := obj.Type().Elem(); t.Kind() == reflect.Ptr {
				conv = func(val tftypes.Value) error {
					e := reflect.New(t.Elem())
					if err := c.AsGo(val, e.Interface()); err != nil {
						return err
					}
					o = reflect.Append(o, e)
					return nil
				}
			} else {
				conv = func(val tftypes.Value) error {
					e := reflect.New(t)
					if err := c.AsGo(val, e.Interface()); err != nil {
						return err
					}
					o = reflect.Append(o, e.Elem())
					return nil
				}
			}

			for _, val := range v {
				if err := conv(val); err != nil {
					return err
				}
			}

			obj.Set(o)
			return nil
		},
		asValue: func(obj interface{}) tftypes.Value {
			val := []tftypes.Value{}

			if obj != nil {
				o := reflect.ValueOf(obj)

				for i := 0; i < o.Len(); i++ {
					v := reflect.Indirect(o.Index(i))
					val = append(val, c.AsValue(v.Interface()))
				}
			}

			return tftypes.NewValue(
				tftypes.List{ElementType: c.AsValue(nil).Type()},
				val,
			)
		},
	}, nil
}

func NewMapBlock(val reflect.Type, goName, valueName string) (*Block, error) {
	c, err := NewConverter(val)
	if err != nil {
		return nil, err
	}

	return &Block{
		goName:    goName,
		valueName: valueName,
		mode:      tfprotov5.SchemaNestedBlockNestingModeMap,
		schema:    c.Schema,
		asGo: func(val tftypes.Value, obj reflect.Value) error {
			if val.IsNull() {
				return nil
			}

			// Since the block has labels, we need to parse each
			// sub-block, add their labels to them, put them in a
			// slice and use that as a value for the object

			var v map[string]tftypes.Value
			err := val.As(&v)
			if err != nil {
				return err
			}

			// We will store each sub-block in this slice
			o := reflect.MakeSlice(obj.Type(), 0, 0)

			setLabel := func(label string, t reflect.Type, e reflect.Value) error {
				fieldName, err := getLabelFieldName(t)
				if err != nil {
					return err
				}

				labelField := e.Elem().FieldByName(fieldName)
				if labelField.Kind() == reflect.Ptr {
					labelField.Set(reflect.ValueOf(&label))
				} else {
					labelField.SetString(label)
				}
				return nil
			}

			conv := func(label string, val tftypes.Value) error {
				t := obj.Type().Elem()
				if t.Kind() == reflect.Ptr {
					t = t.Elem()
				}
				e := reflect.New(t)
				if err := setLabel(label, t, e); err != nil {
					return err
				}
				if err := c.AsGo(val, e.Interface()); err != nil {
					return err
				}
				if obj.Type().Elem().Kind() == reflect.Ptr {
					o = reflect.Append(o, e)
				} else {
					o = reflect.Append(o, e.Elem())
				}
				return nil
			}

			for label, val := range v {
				if err := conv(label, val); err != nil {
					return err
				}
			}

			obj.Set(o)
			return nil
		},
		asValue: func(obj interface{}) tftypes.Value {
			val := map[string]tftypes.Value{}

			if obj != nil {
				o := reflect.ValueOf(obj)

				for i := 0; i < o.Len(); i++ {
					v := reflect.Indirect(o.Index(i))
					fieldName, _ := getLabelFieldName(v.Type())

					labelField := v.FieldByName(fieldName)

					e := c.AsValue(v.Interface())
					val[reflect.Indirect(labelField).String()] = e
				}
			}

			return tftypes.NewValue(
				tftypes.Map{
					AttributeType: c.AsValue(nil).Type(),
				},
				val,
			)
		},
	}, nil
}
