package controls

import (
	"os"
	"testing"

	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/goccy/go-yaml"
	"github.com/oscal-compass/oscal-sdk-go/validation"
	"github.com/ossf/gemara/layer1"
	"github.com/stretchr/testify/require"
)

func TestToCatalog(t *testing.T) {
	file, err := os.Open("./testdata/800-161.yml")
	require.NoError(t, err)

	var guidance layer1.GuidanceDocument
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&guidance)
	require.NoError(t, err)

	catalog, err := ToCatalog(guidance)
	require.NoError(t, err)

	oscalModels := oscalTypes.OscalModels{
		Catalog: &catalog,
	}

	validator := validation.NewSchemaValidator()
	err = validator.Validate(oscalModels)
	require.NoError(t, err)
}
