package stos

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

func MapStructToStruct(interfaceType interface{}) error {
	typ := reflect.TypeOf(interfaceType)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Interface {
		return fmt.Errorf("expected an interface, got %s", typ.Kind())
	}

	pkgPath := typ.PkgPath()
	if pkgPath == "" {
		return fmt.Errorf("unable to determine package path for interface %v", typ)
	}

	pkgDir := getPackageDir(pkgPath)
	if pkgDir == "" {
		return fmt.Errorf("unable to locate package directory for %s", pkgPath)
	}

	gen := &MapperGenerator{
		InterfaceType: typ,
		PackagePath:   pkgPath,
		PackageDir:    pkgDir,
		Generated:     make(map[string]bool),
		HelperMethods: []string{},
	}

	return gen.writeToFile()
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

	for i := 0; i < sourceType.NumField(); i++ {
		sourceField := sourceType.Field(i)
		if targetField, found := targetType.FieldByName(sourceField.Name); found {
			// Handle pointer and non-pointer variations
			mappingCode := g.generateFieldMapping(sourceField, targetField)
			if mappingCode != "" {
				sb.WriteString(mappingCode)
			}
		}
	}
	return sb.String()
}

func (g *MapperGenerator) generateFieldMapping(sourceField, targetField reflect.StructField) string {
	sourceType := sourceField.Type
	targetType := targetField.Type

	// Direct assignment for exact type match
	if sourceType == targetType {
		return fmt.Sprintf("\tobjTarget.%s = objSource.%s\n", targetField.Name, sourceField.Name)
	}

	// Pointer to pointer handling
	if sourceType.Kind() == reflect.Ptr && targetType.Kind() == reflect.Ptr {
		sourcePtrType := sourceType.Elem()
		targetPtrType := targetType.Elem()

		if sourcePtrType.Kind() == reflect.Struct && targetPtrType.Kind() == reflect.Struct {
			methodName := fmt.Sprintf("map%sTo%s", sourcePtrType.Name(), targetPtrType.Name())
			if !g.Generated[methodName] {
				g.Generated[methodName] = true
				g.addHelperMethod(sourcePtrType, targetPtrType)
			}

			return fmt.Sprintf(`	// Pointer to pointer mapping with nil check
	if objSource.%s != nil {
		tempVal := impl.%s(*objSource.%s)
		objTarget.%s = &tempVal
	} else {
		objTarget.%s = nil
	}
`, sourceField.Name, methodName, sourceField.Name, targetField.Name, targetField.Name)
		}

		// Simple pointer-to-pointer copy for non-struct types
		return fmt.Sprintf(`	// Pointer to pointer simple copy with nil check
	if objSource.%s != nil {
		tempVal := *objSource.%s
		objTarget.%s = &tempVal
	} else {
		objTarget.%s = nil
	}
`, sourceField.Name, sourceField.Name, targetField.Name, targetField.Name)
	}

	// Non-pointer to pointer handling
	if sourceType.Kind() != reflect.Ptr && targetType.Kind() == reflect.Ptr {
		targetPtrType := targetType.Elem()

		if sourceType.Kind() == reflect.Struct && targetPtrType.Kind() == reflect.Struct {
			methodName := fmt.Sprintf("map%sTo%s", sourceType.Name(), targetPtrType.Name())
			if !g.Generated[methodName] {
				g.Generated[methodName] = true
				g.addHelperMethod(sourceType, targetPtrType)
			}

			return fmt.Sprintf(`	// Non-pointer to pointer mapping
	tempVal := impl.%s(objSource.%s)
	objTarget.%s = &tempVal
`, methodName, sourceField.Name, targetField.Name)
		}

		// Simple non-pointer to pointer copy
		return fmt.Sprintf(`	// Non-pointer to pointer simple copy
	tempVal := objSource.%s
	objTarget.%s = &tempVal
`, sourceField.Name, targetField.Name)
	}

	// Pointer to non-pointer handling
	if sourceType.Kind() == reflect.Ptr && targetType.Kind() != reflect.Ptr {
		sourcePtrType := sourceType.Elem()

		if sourcePtrType.Kind() == reflect.Struct && targetType.Kind() == reflect.Struct {
			methodName := fmt.Sprintf("map%sTo%s", sourcePtrType.Name(), targetType.Name())
			if !g.Generated[methodName] {
				g.Generated[methodName] = true
				g.addHelperMethod(sourcePtrType, targetType)
			}

			return fmt.Sprintf(`	// Pointer to non-pointer mapping with nil check
	if objSource.%s != nil {
		objTarget.%s = impl.%s(*objSource.%s)
	} else {
		// Handle nil pointer case (use zero value or skip)
		var zeroValue %s
		objTarget.%s = zeroValue
	}
`, sourceField.Name, targetField.Name, methodName, sourceField.Name, targetType.String(), targetField.Name)
		}

		// Simple pointer to non-pointer copy
		return fmt.Sprintf(`	// Pointer to non-pointer simple copy with nil check
	if objSource.%s != nil {
		objTarget.%s = *objSource.%s
	} else {
		// Handle nil pointer case (use zero value)
		var zeroValue %s
		objTarget.%s = zeroValue
	}
`, sourceField.Name, targetField.Name, sourceField.Name, targetType.String(), targetField.Name)
	}

	// Slice handling with element mapping
	if sourceType.Kind() == reflect.Slice && targetType.Kind() == reflect.Slice {
		sourceElemType := sourceType.Elem()
		targetElemType := targetType.Elem()

		if sourceElemType.Kind() == reflect.Struct && targetElemType.Kind() == reflect.Struct {
			methodName := fmt.Sprintf("map%sTo%s", sourceElemType.Name(), targetElemType.Name())
			if !g.Generated[methodName] {
				g.Generated[methodName] = true
				g.addHelperMethod(sourceElemType, targetElemType)
			}

			return fmt.Sprintf("\t// Slice mapping with element type conversion\nif len(objSource.%s) > 0 {\n\tobjTarget.%s = make([]%s, len(objSource.%s))\n\tfor i, v := range objSource.%s {\n\t\tobjTarget.%s[i] = impl.%s(v)\n\t}\n}\n", sourceField.Name, targetField.Name, targetElemType.String(), sourceField.Name, sourceField.Name, targetField.Name, methodName)
		}

		if sourceElemType.Kind() == reflect.Ptr && targetElemType.Kind() == reflect.Struct {
			methodName := fmt.Sprintf("map%sTo%s", sourceElemType.Elem().Name(), targetElemType.Name())
			if !g.Generated[methodName] {
				g.Generated[methodName] = true
				g.addHelperMethod(sourceElemType.Elem(), targetElemType)
			}

			return fmt.Sprintf("\t// Slice of pointers to slice of structs mapping\nif len(objSource.%s) > 0 {\n\tobjTarget.%s = make([]%s, len(objSource.%s))\n\tfor i, v := range objSource.%s {\n\t\tif v != nil {\n\t\t\tobjTarget.%s[i] = impl.%s(*v)\n\t\t}\n\t}\n}\n", sourceField.Name, targetField.Name, targetElemType.String(), sourceField.Name, sourceField.Name, targetField.Name, methodName)
		}

		if sourceElemType.Kind() == reflect.Struct && targetElemType.Kind() == reflect.Ptr {
			methodName := fmt.Sprintf("map%sTo%s", sourceElemType.Name(), targetElemType.Elem().Name())
			if !g.Generated[methodName] {
				g.Generated[methodName] = true
				g.addHelperMethod(sourceElemType, targetElemType.Elem())
			}

			return fmt.Sprintf("\t// Slice of structs to slice of pointers mapping\nif len(objSource.%s) > 0 {\n\tobjTarget.%s = make([]*%s, len(objSource.%s))\n\tfor i, v := range objSource.%s {\n\t\ttempVal := impl.%s(v)\n\t\tobjTarget.%s[i] = &tempVal\n\t}\n}\n", sourceField.Name, targetField.Name, targetElemType.Elem().String(), sourceField.Name, sourceField.Name, methodName, targetField.Name)
		}

		if sourceElemType.Kind() == reflect.Ptr && targetElemType.Kind() == reflect.Ptr {
			methodName := fmt.Sprintf("map%sTo%s", sourceElemType.Elem().Name(), targetElemType.Elem().Name())
			if !g.Generated[methodName] {
				g.Generated[methodName] = true
				g.addHelperMethod(sourceElemType.Elem(), targetElemType.Elem())
			}

			return fmt.Sprintf("\t// Slice of pointers to slice of pointers mapping\nif len(objSource.%s) > 0 {\n\tobjTarget.%s = make([]*%s, len(objSource.%s))\n\tfor i, v := range objSource.%s {\n\t\tif v != nil {\n\t\t\ttempVal := impl.%s(*v)\n\t\t\tobjTarget.%s[i] = &tempVal\n\t\t}\n\t}\n}\n", sourceField.Name, targetField.Name, targetElemType.Elem().String(), sourceField.Name, sourceField.Name, methodName, targetField.Name)
		}
	}

	// Default case: add a comment for unhandled mappings
	return fmt.Sprintf("\t// TODO: Unhandled mapping for %s -> %s\n", sourceType.String(), targetType.String())
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

func (g *MapperGenerator) generateCode() (string, error) {
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

func (g *MapperGenerator) writeToFile() error {
	code, err := g.generateCode()
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
