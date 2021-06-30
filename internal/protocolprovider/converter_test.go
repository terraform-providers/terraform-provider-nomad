package protocol

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
)

// It is important for the conversion from HCL annotation to Terraform schema
// to be correct, since it would be extremely tedious to test all possible jobs
// configuration to make sure we cover all cases, we just use this struct in
// which all the types covered in the coverters are present. If it works
// properly for this struct, it should work without any issue for api.Job.
type TestStruct struct {
	// Strings
	String         string            `hcl:"string,optional"`
	StringPtr      *string           `hcl:"stringptr,optional"`
	StringPtrNil   *string           `hcl:"stringptrnil,optional"`
	Duration       time.Duration     `hcl:"duration,optional"`
	DurationPtr    *time.Duration    `hcl:"durationptr,optional"`
	DurationPtrNil *time.Duration    `hcl:"durationptrnil,optional"`
	CSIPluginType  api.CSIPluginType `hcl:"csiplugintype,optional"`

	// Numbers
	Int          int     `hcl:"int,optional"`
	IntPtr       *int    `hcl:"intptr,optional"`
	IntPtrNil    *int    `hcl:"intptrnil,optional"`
	Int8Ptr      *int8   `hcl:"int8ptr,optional"`
	Int8PtrNil   *int8   `hcl:"int8ptrnil,optional"`
	UInt64Ptr    *uint64 `hcl:"uint64ptr,optional"`
	UInt64PtrNil *uint64 `hcl:"uint64ptrnil,optional"`
	Int64Ptr     *int64  `hcl:"int64ptr,optional"`
	Int64PtrNil  *int64  `hcl:"int64ptrnil,optional"`
	UInt8        uint8   `hcl:"uint8,optional"`

	// Bools
	Bool       bool  `hcl:"bool,optional"`
	BoolPtr    *bool `hcl:"boolptr,optional"`
	BoolPtrNil *bool `hcl:"boolptrnil,optional"`

	// Complex attributes
	ListOfStrings                     []string                                 `hcl:"listofstrings,optional"`
	MapStringInterface                map[string]interface{}                   `hcl:"mapstringinterface,optional"`
	MapStringString                   map[string]string                        `hcl:"mapstringstring,optional"`
	MapStringListOfString             map[string][]string                      `hcl:"mapstringlistofstring,optional"`
	MapStringVolumeRequest            map[string]*api.VolumeRequest            `hcl:"mapstringvolumerequest,optional"`
	MapStringConsulGatewayBindAddress map[string]*api.ConsulGatewayBindAddress `hcl:"mapstringconsulgatewaybindaddress,optional"`
}

func TestConverterSchema(t *testing.T) {
	ty := reflect.TypeOf(TestStruct{})
	converter, err := NewConverter(ty)
	require.NoError(t, err)

	expected := &tfprotov5.SchemaBlock{
		Version: 1,
		Attributes: []*tfprotov5.SchemaAttribute{
			{
				Name:     "bool",
				Type:     tftypes.Bool,
				Optional: true,
			},
			{
				Name:     "boolptr",
				Type:     tftypes.Bool,
				Optional: true,
			},
			{
				Name:     "boolptrnil",
				Type:     tftypes.Bool,
				Optional: true,
			},
			{
				Name:     "csiplugintype",
				Type:     tftypes.String,
				Optional: true,
			},
			{
				Name:     "duration",
				Type:     tftypes.String,
				Optional: true,
			},
			{
				Name:     "durationptr",
				Type:     tftypes.String,
				Optional: true,
			},
			{
				Name:     "durationptrnil",
				Type:     tftypes.String,
				Optional: true,
			},
			{
				Name:     "int",
				Type:     tftypes.Number,
				Optional: true,
			},
			{
				Name:     "int64ptr",
				Type:     tftypes.Number,
				Optional: true,
			},
			{
				Name:     "int64ptrnil",
				Type:     tftypes.Number,
				Optional: true,
			},
			{
				Name:     "int8ptr",
				Type:     tftypes.Number,
				Optional: true,
			},
			{
				Name:     "int8ptrnil",
				Type:     tftypes.Number,
				Optional: true,
			},
			{
				Name:     "intptr",
				Type:     tftypes.Number,
				Optional: true,
			},
			{
				Name:     "intptrnil",
				Type:     tftypes.Number,
				Optional: true,
			},
			{
				Name: "listofstrings",
				Type: tftypes.List{
					ElementType: tftypes.String,
				},
				Optional: true,
			},
			{
				Name: "mapstringconsulgatewaybindaddress",
				Type: tftypes.Map{
					AttributeType: tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"address": tftypes.String,
							"port":    tftypes.Number,
						},
					},
				},
				Optional: true,
			},
			{
				Name:     "mapstringinterface",
				Type:     tftypes.String,
				Optional: true,
			},
			{
				Name: "mapstringlistofstring",
				Type: tftypes.Map{
					AttributeType: tftypes.List{
						ElementType: tftypes.String,
					},
				},
				Optional: true,
			},
			{
				Name: "mapstringstring",
				Type: tftypes.Map{
					AttributeType: tftypes.String,
				},
				Optional: true,
			},
			{
				Name: "mapstringvolumerequest",
				Type: tftypes.Map{
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
				},
				Optional: true,
			},
			{
				Name:     "string",
				Type:     tftypes.String,
				Optional: true,
			},
			{
				Name:     "stringptr",
				Type:     tftypes.String,
				Optional: true,
			},
			{
				Name:     "stringptrnil",
				Type:     tftypes.String,
				Optional: true,
			},
			{
				Name:     "uint64ptr",
				Type:     tftypes.Number,
				Optional: true,
			},
			{
				Name:     "uint64ptrnil",
				Type:     tftypes.Number,
				Optional: true,
			},
			{
				Name:     "uint8",
				Type:     tftypes.Number,
				Optional: true,
			},
		},
		BlockTypes: nil,
	}

	require.Equal(t, expected, converter.Schema)

	// Make sure we have one attribute for each field in TestStruct
	require.Equal(t, len(expected.Attributes), ty.NumField())

	// Make sure we tested all attributes type
	tested := map[string]struct{}{}
	for i := 0; i < ty.NumField(); i++ {
		field := ty.Field(i)
		tested[field.Type.String()] = struct{}{}
	}
	missing := []string{}
	for key := range attributeBuilder {
		if _, ok := tested[key]; !ok {
			missing = append(missing, key)
		}
	}

	// Make sure we have a Nil and a non Nil version of each pointers for the
	// next test
	found := map[string]struct {
		Nil    bool
		NonNil bool
	}{}
	for i := 0; i < ty.NumField(); i++ {
		field := ty.Field(i)
		if field.Type.Kind() == reflect.Ptr {
			name := strings.ReplaceAll(field.Name, "Nil", "")
			v := found[name]
			if strings.HasSuffix(field.Name, "Nil") {
				v.Nil = true
			} else {
				v.NonNil = true
			}
			found[name] = v
		}
	}
	for name, result := range found {
		if !result.Nil || !result.NonNil {
			t.Fatalf("Missing some version for %q: %+v", name, result)
		}
	}

	require.Emptyf(t, missing, "Some types have not been tested")
}

func TestConverterRoundtrip(t *testing.T) {
	s := "foo"
	d, err := time.ParseDuration("1m")
	require.NoError(t, err)
	i := 1
	i8 := int8(8)
	ui8 := uint8(8)
	ui64 := uint64(64)
	i64 := int64(64)
	b := true

	obj := TestStruct{
		String:        s,
		StringPtr:     &s,
		Duration:      d,
		DurationPtr:   &d,
		CSIPluginType: api.CSIPluginTypeNode,
		Int:           i,
		IntPtr:        &i,
		Int8Ptr:       &i8,
		UInt64Ptr:     &ui64,
		Int64Ptr:      &i64,
		UInt8:         ui8,
		Bool:          b,
		BoolPtr:       &b,
		MapStringInterface: map[string]interface{}{
			"driver": "test",
			"port":   []interface{}{float64(1), float64(2)},
		},
		MapStringVolumeRequest: map[string]*api.VolumeRequest{
			"test": {
				Name: "test",
				Type: "foo",
			},
		},
		ListOfStrings:         []string{},
		MapStringString:       map[string]string{},
		MapStringListOfString: map[string][]string{},
		MapStringConsulGatewayBindAddress: map[string]*api.ConsulGatewayBindAddress{
			"test": {
				Name:    "test",
				Address: "foo",
				Port:    80,
			},
		},
	}
	converter, err := NewConverter(reflect.TypeOf(obj))
	require.NoError(t, err)

	// Make sure our conversion roundtrip
	var obj2 TestStruct
	err = converter.AsGo(converter.AsValue(obj), &obj2)
	require.NoError(t, err)
	require.Equal(t, obj, obj2)

	// Make sure our object had correct values to test all code paths
	o := reflect.ValueOf(obj)
	for i := 0; i < o.NumField(); i++ {
		name := o.Type().Field(i).Name
		field := o.FieldByName(name)
		if field.Kind() == reflect.Ptr {
			if strings.HasSuffix(name, "Nil") {
				require.True(t, field.IsNil(), "pointer %q must be nil", name)
			} else {
				require.False(t, field.IsNil(), "pointer %q must be non-nil", name)
				require.False(t, reflect.Indirect(field).IsZero(), "value pointed by %q should not be the default", name)
			}
		} else {
			require.False(t, field.IsZero(), "field %q should not be set to their default value", name)
		}
	}
}
