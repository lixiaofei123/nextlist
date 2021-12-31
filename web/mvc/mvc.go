package mvc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type Result interface {
	Dispatch(ctx echo.Context)
}

type ExtraRouter interface {
	AddRouter(method, path, methodName string)
	GetItems() []RouterItem
}

type RouterItem struct {
	method   string
	path     string
	funcName string
}

type extraRouter struct {
	items []RouterItem
}

func (e *extraRouter) AddRouter(method, path, funcName string) {
	e.items = append(e.items, RouterItem{
		method:   method,
		path:     path,
		funcName: funcName,
	})
}

func (e *extraRouter) GetItems() []RouterItem {
	return e.items
}

func NewExtraRouter() ExtraRouter {
	return &extraRouter{
		items: []RouterItem{},
	}
}

type RegisterRouter interface {
	RegisterRouter(routers ExtraRouter)
}

type Application struct {
	r *echo.Group
}

func New(r *echo.Group) *Application {
	return &Application{
		r: r,
	}
}

var echoContextType reflect.Type = reflect.TypeOf((*echo.Context)(nil)).Elem()
var resultType reflect.Type = reflect.TypeOf((*Result)(nil)).Elem()
var pathRegex = regexp.MustCompile(`([A-Z][a-z0-9]*)`)

func GetRouterPath(matches []string, method reflect.Method) string {
	path := ""
	if len(matches) == 0 || matches[len(matches)-1] != "By" {
		path = fmt.Sprintf("/%s", strings.Join(matches, "/"))
	} else {
		path = strings.Join(matches[:len(matches)-1], "/")
		paramIndex := 0
		for j := 0; j < method.Type.NumIn(); j++ {
			field := method.Type.In(j)
			if field.Kind() >= reflect.Int && field.Kind() <= reflect.Int64 ||
				field.Kind() == reflect.String {
				path = fmt.Sprintf("/%s/:param%d", path, paramIndex)
				paramIndex++
			}
		}
	}

	path = strings.TrimRight(path, "/")
	path = strings.TrimLeft(path, "/")
	if path != "" {
		path = "/" + path
	}

	return path
}

func HandlerExtraRouterFunc(controller interface{}, method reflect.Method) echo.HandlerFunc {
	return func(c echo.Context) error {

		if method.Type.NumIn() > 2 {
			return errors.New("添加额外路由的函数参数最多只能有一个")
		}

		if method.Type.NumIn() == 2 && !method.Type.In(1).Implements(echoContextType) {
			return errors.New("添加额外路由的函数参数的参数只能是echo.Context类型")
		}

		// 获取参数列表
		args := make([]reflect.Value, method.Type.NumIn())

		field := reflect.TypeOf(controller)
		argValue := reflect.ValueOf(controller)
		args[0] = argValue.Convert(field)

		if c.Param("*") != "" {
			path := c.Param("*")
			c.SetParamNames(append([]string{"path"}, c.ParamNames()...)...)
			c.SetParamValues(append([]string{path}, c.ParamValues()...)...)

		}

		if method.Type.NumIn() == 2 {
			field := method.Type.In(1)
			argValue := reflect.ValueOf(c)
			args[1] = argValue.Convert(field)
		}

		returnValue := method.Func.Call(args)[0]
		if returnValue.Interface() != nil {
			if returnValue.Type().Implements(resultType) {
				mvcResult := returnValue.Interface().(Result)
				mvcResult.Dispatch(c)
			} else {
				c.JSON(http.StatusOK, returnValue.Interface())
			}
		}

		return nil
	}
}

func HandlerFunc(controller interface{}, method reflect.Method) echo.HandlerFunc {
	return func(c echo.Context) error {
		// 获取参数列表
		args := make([]reflect.Value, method.Type.NumIn())

		field := reflect.TypeOf(controller)
		argValue := reflect.ValueOf(controller)
		args[0] = argValue.Convert(field)

		paramIndex := 0

		for j := 1; j < method.Type.NumIn(); j++ {
			field := method.Type.In(j)
			if field.Implements(echoContextType) {
				argValue := reflect.ValueOf(c)
				args[j] = argValue.Convert(field)
				continue
			}
			if field.Kind() == reflect.Struct {
				defer c.Request().Body.Close()
				data, err := ioutil.ReadAll(c.Request().Body)
				if err != nil {
					return err
				}
				value := reflect.New(field).Interface()
				err = json.Unmarshal(data, value)
				if err != nil {
					return err
				}
				args[j] = reflect.ValueOf(value).Elem()
				continue

			}
			if field.Kind() >= reflect.Int && field.Kind() <= reflect.Int64 {
				key := fmt.Sprintf("param%d", paramIndex)
				value, _ := strconv.Atoi(c.Param(key))
				argValue := reflect.ValueOf(value)
				args[j] = argValue.Convert(field)
				paramIndex++
			}

			if field.Kind() == reflect.String {
				value := c.Param(fmt.Sprintf("param%d", paramIndex))
				argValue := reflect.ValueOf(value)
				args[j] = argValue.Convert(field)
				paramIndex++
			}
		}

		returnValue := method.Func.Call(args)[0]
		if returnValue.Interface() != nil {
			if returnValue.Type().Implements(resultType) {
				mvcResult := returnValue.Interface().(Result)
				mvcResult.Dispatch(c)
			} else {
				c.JSON(http.StatusOK, returnValue.Interface())
			}
		}

		return nil
	}
}

func (app *Application) Handle(controller interface{}) {
	st := reflect.TypeOf(controller)
	for i := 0; i < st.NumMethod(); i++ {
		method := st.Method(i)
		methodName := method.Name
		matches := pathRegex.FindAllString(methodName, -1)
		if len(matches) > 0 {
			if matches[0] == "Get" || matches[0] == "Post" || matches[0] == "Put" || matches[0] == "Delete" {
				path := strings.ToLower(GetRouterPath(matches[1:], method))
				if matches[0] == "Get" {
					app.r.GET(path, HandlerFunc(controller, method))
				}
				if matches[0] == "Post" {
					app.r.POST(path, HandlerFunc(controller, method))
				}
				if matches[0] == "Put" {
					app.r.PUT(path, HandlerFunc(controller, method))
				}
				if matches[0] == "Delete" {
					app.r.DELETE(path, HandlerFunc(controller, method))
				}

			}
		}

	}

	if registerRouter, ok := controller.(RegisterRouter); ok {
		//registerRouter.RegisterRouter()

		extraRouter := NewExtraRouter()
		registerRouter.RegisterRouter(extraRouter)
		for _, item := range extraRouter.GetItems() {
			if item.method == "Get" || item.method == "Post" ||
				item.method == "Delete" || item.method == "Put" {
				path := item.path
				if strings.HasSuffix(path, ":path") {
					path = strings.Replace(path, ":path", "*", len(path)-6)
				}

				if method, ok := st.MethodByName(item.funcName); ok {
					if item.method == "Get" {
						app.r.GET(path, HandlerExtraRouterFunc(controller, method))
					}
					if item.method == "Post" {
						app.r.POST(path, HandlerExtraRouterFunc(controller, method))
					}
					if item.method == "Put" {
						app.r.PUT(path, HandlerExtraRouterFunc(controller, method))
					}
					if item.method == "Delete" {
						app.r.DELETE(path, HandlerExtraRouterFunc(controller, method))
					}
				}

			}
		}
	}
}
