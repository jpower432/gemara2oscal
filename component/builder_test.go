package component

import (
	"os"
	"testing"

	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/goccy/go-yaml"
	"github.com/oscal-compass/oscal-sdk-go/validation"
	"github.com/revanite-io/sci/layer2"
	"github.com/revanite-io/sci/layer4"
	"github.com/stretchr/testify/require"
)

func TestDefinitionBuilder_Build(t *testing.T) {

	file, err := os.Open("./testdata/good-ccc.yml")
	require.NoError(t, err)

	var catalog layer2.Catalog
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&catalog)
	require.NoError(t, err)

	eval := layer4.ControlEvaluation{
		Control_Id: "CCC.C01",
		Assessments: []*layer4.Assessment{
			{
				Requirement_Id: "CCC.C01.TR01",
				Description:    "my-check-id",
			},
		},
	}

	builder := NewDefinitionBuilder("ComponentDefinition", "v0.1.0")
	componentDefintion := builder.AddTargetComponent("Example", "software", catalog).AddValidationComponent("myvalidator", []layer4.ControlEvaluation{eval}).Build()
	require.Len(t, *componentDefintion.Components, 2)

	components := *componentDefintion.Components
	require.Len(t, *components[0].Props, 20)
	require.Len(t, *components[1].Props, 3)

	oscalModels := oscalTypes.OscalModels{
		ComponentDefinition: &componentDefintion,
	}

	validator := validation.NewSchemaValidator()
	err = validator.Validate(oscalModels)
	require.NoError(t, err)
}
