package main

import (
	"fmt"
	"go/build"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// MapperGenerator generates struct-to-struct mapping code
type MapperGenerator struct {
	InterfaceType reflect.Type
	PackagePath   string
	PackageDir    string
	Generated     map[string]bool
	HelperMethods []string
}

// NewMapperGenerator creates a new instance of MapperGenerator
func NewMapperGenerator(interfaceType interface{}) (*MapperGenerator, error) {
	typ := reflect.TypeOf(interfaceType)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Interface {
		return nil, fmt.Errorf("expected an interface, got %s", typ.Kind())
	}

	pkgPath := typ.PkgPath()
	if pkgPath == "" {
		return nil, fmt.Errorf("unable to determine package path for interface %v", typ)
	}

	pkgDir := getPackageDir(pkgPath)
	if pkgDir == "" {
		return nil, fmt.Errorf("unable to locate package directory for %s", pkgPath)
	}

	return &MapperGenerator{
		InterfaceType: typ,
		PackagePath:   pkgPath,
		PackageDir:    pkgDir,
		Generated:     make(map[string]bool),
		HelperMethods: []string{},
	}, nil
}

func getPackageDir(pkgPath string) string {
	pkg, err := build.Import(pkgPath, ".", build.FindOnly)
	if err != nil {
		log.Fatalf("failed to resolve package directory for %s: %v", pkgPath, err)
	}
	return pkg.Dir
}

func (g *MapperGenerator) generateConstructorStub(interfaceName string) string {
	titleInterface := strings.ToLower(interfaceName[:1]) + interfaceName[1:]
	return fmt.Sprintf("func New%sImpl() %s {\n\treturn &%sImpl{}\n}\n\n", interfaceName, interfaceName, titleInterface)
}

func (g *MapperGenerator) generateMethodStub(method reflect.Method) string {
	var sb strings.Builder
	methodName := method.Name
	if method.Type.NumIn() < 1 || method.Type.NumOut() < 1 {
		log.Fatalf("Method %s must have one input parameter and one output parameter", methodName)
	}

	sourceType := method.Type.In(0)
	targetType := method.Type.Out(0)
	implName := g.InterfaceType.Name()

	sb.WriteString(fmt.Sprintf("func (impl *%sImpl) %s(objSource %s) %s {\n",
		strings.ToLower(implName[:1])+implName[1:], methodName, sourceType.String(), targetType.String()))

	if sourceType.Kind() == reflect.Ptr {
		sb.WriteString("\tif objSource == nil {\n\t\treturn nil\n\t}\n")
	}

	if sourceType.Kind() == reflect.Slice || sourceType.Kind() == reflect.Array {
		sb.WriteString("\tif len(objSource) == 0 {\n\t\treturn nil\n\t}\n")
		sb.WriteString("\tobjTarget := make([]" + targetType.Elem().String() + ", len(objSource))\n")
		sb.WriteString("\tfor i, v := range objSource {\n")
		sb.WriteString("\t\tobjTarget[i] = impl.map" + sourceType.Elem().Name() + "To" + targetType.Elem().Name() + "(v)\n")
		sb.WriteString("\t}\n")
		sb.WriteString("\treturn objTarget\n")
	} else {
		sb.WriteString("\tobjTarget := " + targetType.String() + "{}\n")
		sb.WriteString(g.generateFieldMappings(sourceType, targetType))
		sb.WriteString("\treturn objTarget\n")
	}

	sb.WriteString("}\n\n")
	return sb.String()
}

func (g *MapperGenerator) generateFieldMappings(sourceType, targetType reflect.Type) string {
	var sb strings.Builder
	if sourceType.Kind() != reflect.Struct || targetType.Kind() != reflect.Struct {
		return "\t// TODO: Non-struct mappings not implemented\n"
	}

	// Iterate over the fields of the source struct
	for i := 0; i < sourceType.NumField(); i++ {
		sourceField := sourceType.Field(i)
		if targetField, found := targetType.FieldByName(sourceField.Name); found {
			// Handle direct field mappings
			if sourceField.Type == targetField.Type {
				sb.WriteString(fmt.Sprintf("\tobjTarget.%s = objSource.%s\n", targetField.Name, sourceField.Name))
			} else if sourceField.Type.Kind() == reflect.Struct && targetField.Type.Kind() == reflect.Struct {
				// For nested structs, generate a helper function if not already generated
				methodName := fmt.Sprintf("map%sTo%s", sourceField.Type.Name(), targetField.Type.Name())
				if !g.Generated[methodName] {
					g.Generated[methodName] = true
					// Store the helper function in a temporary list to be added at the end of the file
					g.addHelperMethod(sourceField.Type, targetField.Type)
				}
				sb.WriteString(fmt.Sprintf("\tobjTarget.%s = impl.%s(objSource.%s)\n", targetField.Name, methodName, sourceField.Name))
			} else if sourceField.Type.Kind() == reflect.Slice && targetField.Type.Kind() == reflect.Slice {
				// Handle slice-to-slice mapping
				sb.WriteString(fmt.Sprintf("\tif len(objSource.%s) > 0 {\n", sourceField.Name))
				sb.WriteString(fmt.Sprintf("\t\tobjTarget.%s = make([]%s, len(objSource.%s))\n", targetField.Name, targetField.Type.Elem().String(), sourceField.Name))
				sb.WriteString(fmt.Sprintf("\t\tfor i, v := range objSource.%s {\n", sourceField.Name))

				// Handle recursive mapping for slice elements
				if sourceField.Type.Elem().Kind() == reflect.Struct && targetField.Type.Elem().Kind() == reflect.Struct {
					methodName := fmt.Sprintf("map%sTo%s", sourceField.Type.Elem().Name(), targetField.Type.Elem().Name())
					if !g.Generated[methodName] {
						g.Generated[methodName] = true
						// Store the helper function for the slice element in a temporary list
						g.addHelperMethod(sourceField.Type.Elem(), targetField.Type.Elem())
					}
					// Instead of direct assignment, call the helper function
					sb.WriteString(fmt.Sprintf("\t\t\tobjTarget.%s[i] = impl.%s(v)\n", targetField.Name, methodName))
				} else {
					// Direct assignment for non-struct elements
					sb.WriteString(fmt.Sprintf("\t\t\tobjTarget.%s[i] = v\n", targetField.Name))
				}
				sb.WriteString("\t\t}\n")
				sb.WriteString("\t}\n")
			}
		}
	}
	return sb.String()
}

func (g *MapperGenerator) addHelperMethod(sourceType, targetType reflect.Type) {
	// Add the helper method for mapping a nested struct
	methodName := fmt.Sprintf("map%sTo%s", sourceType.Name(), targetType.Name())
	helperMethod := fmt.Sprintf("func (impl *%sImpl) %s(objSource %s) %s {\n\tobjTarget := %s{}\n%s\n\treturn objTarget\n}\n\n",
		strings.ToLower(g.InterfaceType.Name()[:1])+g.InterfaceType.Name()[1:],
		methodName, sourceType.String(), targetType.String(),
		targetType.String(), g.generateFieldMappings(sourceType, targetType))

	// Store the helper method in a temporary list
	g.Generated[methodName] = true
	g.HelperMethods = append(g.HelperMethods, helperMethod)
}

func (g *MapperGenerator) GenerateCode() (string, error) {
	var sb strings.Builder

	interfaceName := g.InterfaceType.Name()
	if interfaceName == "" {
		return "", fmt.Errorf("interface must have a name")
	}

	sb.WriteString(fmt.Sprintf("package %s\n\n", g.getPackageName()))
	sb.WriteString(g.generateImports())
	sb.WriteString(fmt.Sprintf("type %sImpl struct {}\n\n", strings.ToLower(interfaceName[:1])+interfaceName[1:]))
	sb.WriteString(g.generateConstructorStub(interfaceName))

	for i := 0; i < g.InterfaceType.NumMethod(); i++ {
		method := g.InterfaceType.Method(i)
		sb.WriteString(g.generateMethodStub(method))
	}

	// Add the helper methods at the end of the file
	for _, helperMethod := range g.HelperMethods {
		sb.WriteString(helperMethod)
	}

	return sb.String(), nil
}

func (g *MapperGenerator) generateNestedMappingMethod(sourceType, targetType reflect.Type) string {
	// Generate a new mapping method for nested struct
	return fmt.Sprintf("func (impl *%sImpl) map%sTo%s(objSource %s) %s {\n\tobjTarget := %s{}\n%s\n\treturn objTarget\n}\n\n",
		strings.ToLower(g.InterfaceType.Name()[:1])+g.InterfaceType.Name()[1:],
		sourceType.Name(), targetType.Name(), sourceType.String(), targetType.String(),
		targetType.String(), g.generateFieldMappings(sourceType, targetType))
}

func (g *MapperGenerator) generateImports() string {
	// Create a set of unique imports
	importSet := map[string]struct{}{}
	for i := 0; i < g.InterfaceType.NumMethod(); i++ {
		method := g.InterfaceType.Method(i)
		for j := 0; j < method.Type.NumIn(); j++ {
			g.addPackageToImportSet(method.Type.In(j), importSet)
		}
		for j := 0; j < method.Type.NumOut(); j++ {
			g.addPackageToImportSet(method.Type.Out(j), importSet)
		}
	}

	// Create a sorted list of imports to ensure stability and avoid duplicates
	var imports []string
	for pkg := range importSet {
		imports = append(imports, fmt.Sprintf("\"%s\"", pkg))
	}

	// Clean up unused imports
	var cleanedImports []string
	for _, imp := range imports {
		if !strings.Contains(imp, "time") { // Remove the "time" package if it's not used
			cleanedImports = append(cleanedImports, imp)
		}
	}
	return "import (\n" + strings.Join(cleanedImports, "\n") + "\n)\n\n"
}

func (g *MapperGenerator) addPackageToImportSet(typ reflect.Type, importSet map[string]struct{}) {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.PkgPath() != "" && typ.PkgPath() != g.PackagePath {
		importSet[typ.PkgPath()] = struct{}{}
	}
	if typ.Kind() == reflect.Struct {
		for i := 0; i < typ.NumField(); i++ {
			g.addPackageToImportSet(typ.Field(i).Type, importSet)
		}
	}
}
func (g *MapperGenerator) getPackageName() string {
	parts := strings.Split(g.PackagePath, "/")
	return parts[len(parts)-1]
}

func (g *MapperGenerator) WriteToFile() error {
	code, err := g.GenerateCode()
	if err != nil {
		return err
	}

	fileName := fmt.Sprintf("%s_mapper.go", strings.ToLower(g.InterfaceType.Name()))
	filePath := filepath.Join(g.PackageDir, fileName)

	if err := os.WriteFile(filePath, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}
	return nil
}
