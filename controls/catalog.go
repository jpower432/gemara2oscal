package controls

import (
	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/revanite-io/sci/layer1"
)

func ToResolvedCatalog(layer1.GuidanceDocument) (oscalTypes.Catalog, error) {
	return oscalTypes.Catalog{}, nil
}
