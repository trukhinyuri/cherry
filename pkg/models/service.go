package models

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"strings"

	"github.com/containerum/cherry"
	"github.com/containerum/cherry/pkg/noicerrs"
	"github.com/dave/jennifer/jen"
)

var (
	_ json.Marshaler = &Service{}
)

type Service struct {
	Name      string
	SID       cherry.ErrSID
	Error     []TOMLerror
	Templates map[string]string
	Keys      map[string]string
}

func (service *Service) Validate() error {
	if service.SID == "" {
		return noicerrs.ErrUndefinedSID()
	}
	service.Name = strings.TrimSpace(service.Name)
	if service.Name == "" {
		return noicerrs.ErrUndefinedPackageName()
	}
	service.Name = cleanPackageName(service.Name)
	err := findConfictingKinds(service.Error)
	if err != nil {
		return err
	}
	for i, tomlerr := range service.Error {
		tomlerr.SID = service.SID
		if tomlerr.Kind == 0 {
			return noicerrs.ErrUndefinedKind()
		}
		if tomlerr.StatusHTTP == 0 {
			return noicerrs.ErrUndefinedStatusHTTP()
		}
		service.Error[i] = tomlerr
	}
	sort.Slice(service.Error, func(i, j int) bool {
		left, right := service.Error[i], service.Error[j]
		return left.Kind < right.Kind
	})
	if service.Keys == nil {
		service.Keys = map[string]string{}
	}
	return nil
}
func (service *Service) GenerateSource(wr io.Writer) error {
	if err := service.Validate(); err != nil {
		return err
	}
	pack := jen.NewFile(service.Name)
	pack.PackageComment("Code generated by noice. DO NOT EDIT.")
	consts := []jen.Code{}
	for templateName, templ := range service.Templates {
		consts = append(consts, jen.Id(templateName).Op("=").Lit(templ))
	}
	pack.Line().Const().Defs(consts...).Line()
	for _, errDecl := range service.Error {
		pack.Line().Add(errDecl.GenerateSource())
	}
	pack.Func().Id("renderTemplate").Params(jen.Id("templText").String()).String().Block(
		jen.Id("buf").Op(":=").Op("&").Qual("bytes", "Buffer").Values(),
		jen.Id("templ").Op(",").Id("err").Op(":=").Qual("text/template", "New").Call(jen.Lit("")).
			Dot("Parse").Call(jen.Id("templText")),
		jen.If(jen.Id("err").Op("!=").Nil()).Block(
			jen.Return(jen.Id("err").Dot("Error").Call()),
		),
		jen.Id("err").Op("=").Id("templ").Dot("Execute").Call(jen.Id("buf"), jen.Lit(service.Keys)),
		jen.If(jen.Id("err").Op("!=").Nil()).Block(
			jen.Return(jen.Id("err").Dot("Error").Call()),
		),
		jen.Return(jen.Id("buf").Dot("String").Call()),
	)
	return pack.Render(wr)
}

func (service *Service) GenerateSourceString() (string, error) {
	buf := &bytes.Buffer{}
	err := service.GenerateSource(buf)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (service *Service) MarshalJSON() ([]byte, error) {
	service.Validate()
	type _JSONadaptor struct {
		Name      string        `json:"name"`
		SID       cherry.ErrSID `json:"sid"`
		Errors    []cherry.Err  `json:"errors"`
		Templates map[string]string
		Keys      map[string]string
	}
	adaptor := _JSONadaptor{
		Name:      service.Name,
		SID:       service.SID,
		Templates: service.Templates,
		Keys:      service.Keys,
		Errors:    make([]cherry.Err, 0, len(service.Error)),
	}
	for _, tomlerr := range service.Error {
		adaptor.Errors = append(adaptor.Errors, *tomlerr.Cherry())
	}
	jsonData, err := json.MarshalIndent(adaptor, "", "  ")
	return jsonData, err
}
