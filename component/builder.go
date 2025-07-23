package component

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/go-oscal/src/pkg/uuid"
	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/oscal-compass/oscal-sdk-go/extensions"
	"github.com/oscal-compass/oscal-sdk-go/models"
	"github.com/ossf/gemara/layer2"
	"github.com/ossf/gemara/layer4"

	"github.com/jpower432/gemara2oscal/internal/utils"
)

// DefinitionBuilder constructs an OSCAL Component Definition from Gemara
// inputs.
type DefinitionBuilder struct {
	title               string
	version             string
	targetComponents    []oscalTypes.DefinedComponent
	validationComponent []oscalTypes.DefinedComponent
}

func NewDefinitionBuilder(title, version string) *DefinitionBuilder {
	return &DefinitionBuilder{
		title:   title,
		version: version,
	}
}

func (c *DefinitionBuilder) AddTargetComponent(targetComponent, componentType string, catalog layer2.Catalog) *DefinitionBuilder {
	mappingSet := make(map[string]oscalTypes.ControlImplementationSet)
	for _, mappingRef := range catalog.Metadata.MappingReferences {
		mappingSet[mappingRef.Id] = oscalTypes.ControlImplementationSet{
			UUID:        uuid.NewUUID(),
			Description: mappingRef.Description,
			Source:      mappingRef.Url,
			Props: &[]oscalTypes.Property{
				{
					Name:  extensions.FrameworkProp,
					Value: mappingRef.Id,
					Ns:    extensions.TrestleNameSpace,
				},
			},
		}
	}

	var componentProps []oscalTypes.Property
	var groupNumber = 00

	for _, family := range catalog.ControlFamilies {
		for _, control := range family.Controls {
			for _, assessment := range control.AssessmentRequirements {
				ruleProps := makeRule(assessment, groupNumber)
				groupNumber += 1
				mapRule(assessment.Id, control.GuidelineMappings, mappingSet)
				componentProps = append(componentProps, ruleProps...)
			}
		}
	}

	controlImplementations := make([]oscalTypes.ControlImplementationSet, 0, len(mappingSet))
	for _, ciSet := range controlImplementations {
		controlImplementations = append(controlImplementations, ciSet)
	}

	component := oscalTypes.DefinedComponent{
		UUID:                   uuid.NewUUID(),
		Title:                  targetComponent,
		Type:                   componentType,
		Props:                  utils.NilIfEmpty(&componentProps),
		ControlImplementations: utils.NilIfEmpty(&controlImplementations),
	}
	c.targetComponents = append(c.targetComponents, component)
	return c
}

func (c *DefinitionBuilder) AddValidationComponent(source string, evaluations []layer4.ControlEvaluation) *DefinitionBuilder {
	var componentProps []oscalTypes.Property
	var groupNumber = 00

	for _, eval := range evaluations {
		for _, assessment := range eval.Assessments {
			checkProps := makeCheck(*assessment, groupNumber)
			groupNumber += 1
			componentProps = append(componentProps, checkProps...)
		}
	}

	component := oscalTypes.DefinedComponent{
		UUID:  uuid.NewUUID(),
		Type:  "validation",
		Title: source,
		Props: utils.NilIfEmpty(&componentProps),
	}
	c.validationComponent = append(c.validationComponent, component)
	return c
}

func (c *DefinitionBuilder) Build() oscalTypes.ComponentDefinition {
	metadata := models.NewSampleMetadata()
	metadata.Title = c.title
	metadata.Version = c.version

	var allComponent []oscalTypes.DefinedComponent
	allComponent = append(allComponent, c.targetComponents...)
	allComponent = append(allComponent, c.validationComponent...)

	return oscalTypes.ComponentDefinition{
		UUID:       uuid.NewUUID(),
		Metadata:   metadata,
		Components: utils.NilIfEmpty(&allComponent),
	}
}

func makeRule(requirement layer2.AssessmentRequirement, groupNumber int) []oscalTypes.Property {
	remark := fmt.Sprintf("rule_set_%d", groupNumber)

	ruleIdProp := oscalTypes.Property{
		Name:    extensions.RuleIdProp,
		Value:   requirement.Id,
		Ns:      extensions.TrestleNameSpace,
		Remarks: remark,
	}

	ruleDescProp := oscalTypes.Property{
		Name:    extensions.RuleDescriptionProp,
		Value:   strings.ReplaceAll(requirement.Text, "\n", "\\n"),
		Ns:      extensions.TrestleNameSpace,
		Remarks: remark,
	}
	return []oscalTypes.Property{
		ruleIdProp,
		ruleDescProp,
	}
}

func makeCheck(assessment layer4.Assessment, groupNumber int) []oscalTypes.Property {
	remark := fmt.Sprintf("rule_set_%d", groupNumber)
	ruleIdProp := oscalTypes.Property{
		Name:    extensions.RuleIdProp,
		Value:   assessment.Requirement_Id,
		Ns:      extensions.TrestleNameSpace,
		Remarks: remark,
	}

	checkIdProp := oscalTypes.Property{
		Name: extensions.CheckIdProp,
		// FIXME: I actually need an ID here, but it doesn't exist right now
		Value:   strings.ReplaceAll(assessment.Description, "\n", "\\n"),
		Ns:      extensions.TrestleNameSpace,
		Remarks: remark,
	}

	checkDescProp := oscalTypes.Property{
		Name:    extensions.CheckDescriptionProp,
		Value:   assessment.Description,
		Ns:      extensions.TrestleNameSpace,
		Remarks: remark,
	}
	return []oscalTypes.Property{
		ruleIdProp,
		checkIdProp,
		checkDescProp,
	}
}

func mapRule(ruleId string, mappings []layer2.Mapping, ciSets map[string]oscalTypes.ControlImplementationSet) {
	ruleIdProp := oscalTypes.Property{
		Name:  extensions.RuleIdProp,
		Value: ruleId,
		Ns:    extensions.TrestleNameSpace,
	}

	for _, mapping := range mappings {
		targetCI := ciSets[mapping.ReferenceId]
		for _, identifier := range mapping.Identifiers {
			createOrUpdateImplementedRequirement(ruleIdProp, identifier, &targetCI)
		}
		ciSets[mapping.ReferenceId] = targetCI
	}
}

func createOrUpdateImplementedRequirement(ruleIdProp oscalTypes.Property, identifier string, controlImplementation *oscalTypes.ControlImplementationSet) {
	var found bool
	for i := range controlImplementation.ImplementedRequirements {
		if controlImplementation.ImplementedRequirements[i].ControlId == identifier {
			if controlImplementation.ImplementedRequirements[i].Props == nil {
				controlImplementation.ImplementedRequirements[i].Props = &[]oscalTypes.Property{}
			}
			*controlImplementation.ImplementedRequirements[i].Props = append(*controlImplementation.ImplementedRequirements[i].Props, ruleIdProp)
			found = true
			break
		}
	}

	// Check if it is set, this means create a new one
	if !found {
		implRequirement := oscalTypes.ImplementedRequirementControlImplementation{
			UUID:      uuid.NewUUID(),
			ControlId: identifier,
			Props:     &[]oscalTypes.Property{ruleIdProp},
		}
		controlImplementation.ImplementedRequirements = append(controlImplementation.ImplementedRequirements, implRequirement)
	}
}
