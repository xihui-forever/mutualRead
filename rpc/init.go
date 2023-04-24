package rpc

import (
	"github.com/bytedance/sonic"
	"github.com/darabuchi/log"
	"github.com/xihui-forever/goon"
	"github.com/xihui-forever/goon/middleware/session"
	"github.com/xihui-forever/mutualRead/role"
	"github.com/xihui-forever/mutualRead/types"
	"reflect"
	"strconv"
)

func Load() {
	goon.PreUse("/", func(ctx *goon.Ctx) error {
		ctx.SetResHeader("Origin", "*")
		ctx.SetResHeader("Access-Control-Allow-Origin", "*")
		ctx.SetResHeader("Access-Control-Allow-Credentials", "true")
		ctx.SetResHeader("Access-Control-Expose-Headers", "")
		ctx.SetResHeader("Access-Control-Allow-Methods", "*")
		ctx.SetResHeader("Access-Control-Allow-Headers", "*")
		if ctx.Method() == "OPTIONS" {
			ctx.SetStatusCode(200)
			return ctx.Send("")
		}
		return ctx.Next()
	})

	goon.PreUse("/", func(ctx *goon.Ctx) error {
		path := ctx.Path()
		flag, err := role.CheckPermission(role.RoleTypePublic, path)
		if flag {
			return ctx.Next()
		}
		/*if err != role.ErrRolePermExists {
			log.Errorf("err:%v", err)
			return err
		}*/

		sessionBuf, err := session.GetSession(ctx.GetReqHeader("X-Session-Id"))
		if err != nil {
			log.Errorf("err:%v", err)
			return err
		}

		var sess types.LoginSession
		err = sonic.UnmarshalString(sessionBuf, &sess)
		if err != nil {
			log.Errorf("err:%v", err)
			return err
		}

		ctx.SetReqHeader(types.HeaderUserId, strconv.FormatUint(sess.Id, 10))

		ctx.Set(types.HeaderUserId, sess.Id)
		ctx.Set(types.HeaderRoleType, sess.RoleType)

		roleType := sess.RoleType
		_, err = role.CheckPermission(roleType, path)
		if err != nil {
			log.Errorf("err:%v", err)
			return err
		}

		return ctx.Next()
	})

	for _, cmd := range CmdList {
		Post(cmd.Path, cmd.Logic)

		for _, r := range cmd.Roles {
			_, _ = role.BatchAddRolePerm(r, []string{cmd.Path})
		}
	}
}

func Post(path string, logic interface{}) {
	goon.Post(path, GenHandler(logic))
}

func HandleError(ctx *goon.Ctx, err error) error {
	if err == nil {
		return ctx.Json(&types.Error{})
	}

	if x, ok := err.(*types.Error); ok {
		return ctx.Json(x)
	}

	return ctx.Json(&types.Error{
		Code: types.SysError,
		Msg:  err.Error(),
	})
}

func GenHandler(logic any) goon.Handler {
	switch logic.(type) {
	case goon.Handler:
		return func(ctx *goon.Ctx) error {
			return HandleError(ctx, logic.(goon.Handler)(ctx))
		}
	default:
		lt := reflect.TypeOf(logic)
		lv := reflect.ValueOf(logic)
		if lt.Kind() != reflect.Func {
			panic("parameter is not func")
		}

		// 不管怎么样，第一个参数都是一定要存在的
		x := lt.In(0)
		for x.Kind() == reflect.Ptr {
			x = x.Elem()
		}
		if x.Kind() != reflect.Struct {
			panic("first in is must *github.com/conuwa/rpc.Ctx")
		}
		if x.Name() != "Ctx" {
			panic("first in is must *github.com/conuwa/rpc.Ctx")
		}
		if x.PkgPath() != "github.com/conuwa/rpc" {
			panic("first in is must *github.com/conuwa/rpc.Ctx")
		}

		// 按照三种不同的情况来处理
		if lt.NumIn() == 1 && lt.NumOut() == 1 {
			// 一个入参，一个出参，那就是Handler本身了
			// 但是这里还要判断一下，出参是否是error

			x = lt.Out(0)
			for x.Kind() == reflect.Ptr {
				x = x.Elem()
			}

			if x.Name() != "error" {
				panic("out is must error")
			}

			return func(ctx *goon.Ctx) error {
				return HandleError(ctx, logic.(goon.Handler)(ctx))
			}
		} else if lt.NumIn() == 2 && lt.NumOut() == 1 {
			// 两个入参，一个出参，那就是需要解析请求参数的

			// 先判断出参是否是error
			x = lt.Out(0)
			for x.Kind() == reflect.Ptr {
				x = x.Elem()
			}
			if x.Name() != "error" {
				panic("out is must error")
			}

			// 处理一下入参
			x = lt.In(1)
			for x.Kind() == reflect.Ptr {
				x = x.Elem()
			}

			if x.Kind() != reflect.Struct {
				panic("2rd in is must struct")
			}

			return func(ctx *goon.Ctx) error {
				req := reflect.New(x)
				err := ctx.ParseBody(&req)
				if err != nil {
					return err
				}

				out := lv.Call([]reflect.Value{reflect.ValueOf(ctx), req})
				if out[0].IsNil() {
					return ctx.Json(&types.Error{})
				}

				return HandleError(ctx, out[0].Interface().(error))
			}
		} else if lt.NumIn() == 1 && lt.NumOut() == 2 {
			// 一个入参，两个出参，那就是需要返回数据的

			// 先判断第二个出参是否是error
			x = lt.Out(1)
			for x.Kind() == reflect.Ptr {
				x = x.Elem()
			}
			if x.Name() != "error" {
				panic("out 1 is must error")
			}

			// 处理一下返回
			x = lt.Out(0)
			for x.Kind() == reflect.Ptr {
				x = x.Elem()
			}
			if x.Kind() != reflect.Struct {
				panic("out 0 is must struct")
			}

			return func(ctx *goon.Ctx) error {
				out := lv.Call([]reflect.Value{reflect.ValueOf(ctx)})
				if out[1].IsNil() {
					return ctx.Json(&types.Error{
						Data: out[0].Interface(),
					})
				}

				return HandleError(ctx, out[1].Interface().(error))
			}
		} else if lt.NumIn() == 2 && lt.NumOut() == 2 {
			// 两个入参，两个出参，那就是需要解析请求参数，返回数据的

			// 先判断第二个出参是否是error
			x = lt.Out(1)
			for x.Kind() == reflect.Ptr {
				x = x.Elem()
			}
			if x.Name() != "error" {
				panic("out 1 is must error")
			}

			// 处理一下入参
			in := lt.In(1)
			for in.Kind() == reflect.Ptr {
				in = in.Elem()
			}
			if in.Kind() != reflect.Struct {
				panic("2rd in is must struct")
			}

			// 处理一下返回
			out := lt.Out(0)
			for out.Kind() == reflect.Ptr {
				out = out.Elem()
			}
			if out.Kind() != reflect.Struct {
				panic("out 0 is must struct")
			}

			return func(ctx *goon.Ctx) error {
				req := reflect.New(in)
				err := ctx.ParseBody(&req)
				if err != nil {
					return err
				}

				out := lv.Call([]reflect.Value{reflect.ValueOf(ctx), req})
				if out[1].IsNil() {
					return ctx.Json(&types.Error{
						Data: out[0].Interface(),
					})
				}

				return HandleError(ctx, out[1].Interface().(error))
			}
		} else {
			panic("func is not support")
		}
	}
}

type Cmd struct {
	Path  string
	Roles []int
	Logic interface{} // func(ctx, req) (resp, err)
}

var CmdList = []Cmd{}

func Register(path string, logic any, roles ...int) {
	CmdList = append(CmdList, Cmd{
		Path:  path,
		Roles: roles,
		Logic: logic,
	})
}