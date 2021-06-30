package protocol

// Some parts of this were copied from https://github.com/paultyng/terraform-provider-sql/blob/main/internal/server/util.go

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// potential terraform-plugin-go convenience funcs
func unmarshalDynamicValueObject(dv *tfprotov5.DynamicValue, ty tftypes.Object) (tftypes.Value, map[string]tftypes.Value, error) {
	obj, err := dv.Unmarshal(ty)
	if err != nil {
		return tftypes.Value{}, nil, fmt.Errorf("error dv.Unmarshal: %w", err)
	}

	objMap := map[string]tftypes.Value{}
	err = obj.As(&objMap)
	if err != nil {
		return tftypes.Value{}, nil, fmt.Errorf("error obj.As: %w", err)
	}

	return obj, objMap, nil
}

func schemaAsObject(schema *tfprotov5.Schema, markAttributesAsOptional bool) tftypes.Object {
	return blockAsObject(schema.Block, markAttributesAsOptional)
}

func blockAsObject(block *tfprotov5.SchemaBlock, markAttributesAsOptional bool) tftypes.Object {
	o := tftypes.Object{
		AttributeTypes:     map[string]tftypes.Type{},
		OptionalAttributes: map[string]struct{}{},
	}

	for _, b := range block.BlockTypes {
		o.AttributeTypes[b.TypeName] = nestedBlockAsObject(b, markAttributesAsOptional)
		if markAttributesAsOptional {
			o.OptionalAttributes[b.TypeName] = struct{}{}
		}
	}

	for _, s := range block.Attributes {
		o.AttributeTypes[s.Name] = s.Type
		if markAttributesAsOptional {
			o.OptionalAttributes[s.Name] = struct{}{}
		}
	}

	return o
}

func nestedBlockAsObject(nestedBlock *tfprotov5.SchemaNestedBlock, markAttributesAsOptional bool) tftypes.Type {
	switch nestedBlock.Nesting {
	case tfprotov5.SchemaNestedBlockNestingModeSingle:
		return blockAsObject(nestedBlock.Block, markAttributesAsOptional)
	case tfprotov5.SchemaNestedBlockNestingModeList:
		return tftypes.List{
			ElementType: blockAsObject(nestedBlock.Block, markAttributesAsOptional),
		}
	case tfprotov5.SchemaNestedBlockNestingModeMap:
		return tftypes.Map{
			AttributeType: blockAsObject(nestedBlock.Block, markAttributesAsOptional),
		}
	}

	panic(fmt.Sprintf("nested type of %s for %s not supported", nestedBlock.Nesting, nestedBlock.TypeName))
}

func wrap(err error, summary string) []*tfprotov5.Diagnostic {
	if err == nil {
		return []*tfprotov5.Diagnostic{}
	}
	return []*tfprotov5.Diagnostic{
		{
			Severity: tfprotov5.DiagnosticSeverityError,
			Summary:  summary,
			Detail:   err.Error(),
		},
	}
}
