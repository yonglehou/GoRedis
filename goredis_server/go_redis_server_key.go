package goredis_server

import (
	. "GoRedis/goredis"
	"GoRedis/libs/levelredis"
	"strings"
)

func (server *GoRedisServer) OnPING(cmd *Command) (reply *Reply) {
	reply = StatusReply("PONG")
	return
}

// 命名上使用keys来提供keysearch的扫描功能比较合理，
// 但是为了防止开发人员错误把适用与GoRedis的keys处理代码误用到官方redis上造成卡死
// 这里还是把keys禁用
func (server *GoRedisServer) OnKEYS(cmd *Command) (reply *Reply) {
	return ErrorReply("keys is not supported by GoRedis, use 'keysearch [prefix] [count] [withtype]' instead")
}

// keyprev [seek] [count] [withtype] [withvalue]
func (server *GoRedisServer) OnKEYPREV(cmd *Command) (reply *Reply) {
	return server.keyEnumerate(cmd, levelredis.IterBackward)
}

// keynext [seek] [count] [withtype] [withvalue]
// 1) [key]
// 2) [type]
// 3) [value]
// 4) [key2]
// 5) [type2]
// 6) [value2]
func (server *GoRedisServer) OnKEYNEXT(cmd *Command) (reply *Reply) {
	return server.keyEnumerate(cmd, levelredis.IterForward)
}

func (server *GoRedisServer) keyEnumerate(cmd *Command, direction levelredis.IterDirection) (reply *Reply) {
	seek := cmd.Args[1]
	count := 1
	withtype := false
	withvalue := false
	argcount := len(cmd.Args)
	if argcount > 2 {
		var err error
		count, err = cmd.IntAtIndex(2)
		if err != nil {
			return ErrorReply(err)
		}
		if count < 1 || count > 10000 {
			return ErrorReply("count range: 1 < count < 10000")
		}
	}
	if argcount > 3 {
		withtype = strings.ToUpper(cmd.StringAtIndex(3)) == "WITHTYPE"
	}
	// 必须withtype才能withvalue
	if withtype && argcount > 4 {
		withvalue = strings.ToUpper(cmd.StringAtIndex(4)) == "WITHVALUE"
	}
	// bulks初始大小
	bufferSize := count
	if withtype {
		bufferSize = count * 2
		if withvalue {
			bufferSize = count * 3
		}
	}
	bulks := make([]interface{}, 0, bufferSize)
	server.levelRedis.KeyEnumerate(seek, direction, func(i int, key, keytype, value []byte, quit *bool) {
		// stdlog.Println(i, string(key), string(keytype), string(value))
		bulks = append(bulks, key)
		if withtype {
			bulks = append(bulks, keytype)
			if withvalue {
				bulks = append(bulks, value)
			}
		}
		if i >= count-1 {
			*quit = true
		}
	})
	return MultiBulksReply(bulks)
}

// 找出下一个key
func (server *GoRedisServer) OnKEYSEARCH(cmd *Command) (reply *Reply) {
	seekkey, err := cmd.ArgAtIndex(1)
	if err != nil {
		return ErrorReply(err)
	}
	count := 1
	if len(cmd.Args) > 2 {
		count, err = cmd.IntAtIndex(2)
		if err != nil {
			return ErrorReply(err)
		}
		if count < 1 || count > 10000 {
			return ErrorReply("count range: 1 < count < 10000")
		}
	}
	withtype := false
	if len(cmd.Args) > 3 {
		withtype = strings.ToUpper(cmd.StringAtIndex(3)) == "WITHTYPE"
	}
	// search
	bulks := make([]interface{}, 0, 10)
	server.levelRedis.Keys(seekkey, func(i int, key, keytype []byte, quit *bool) {
		bulks = append(bulks, key)
		if withtype {
			bulks = append(bulks, keytype)
		}
		if i >= count-1 {
			*quit = true
		}
	})
	return MultiBulksReply(bulks)
}

// 扫描内部key
func (server *GoRedisServer) OnRAW_KEYSEARCH(cmd *Command) (reply *Reply) {
	seekkey, err := cmd.ArgAtIndex(1)
	if err != nil {
		return ErrorReply(err)
	}
	count := 1
	if len(cmd.Args) > 2 {
		count, err = cmd.IntAtIndex(2)
		if err != nil {
			return ErrorReply(err)
		}
		if count < 1 || count > 10000 {
			return ErrorReply("count range: 1 < count < 10000")
		}
	}
	// search
	bulks := make([]interface{}, 0, 10)
	min := seekkey
	max := append(seekkey, 254)
	server.levelRedis.RangeEnumerate(min, max, levelredis.IterForward, func(i int, key, value []byte, quit *bool) {
		bulks = append(bulks, key)
		if i >= count-1 {
			*quit = true
		}
	})
	return MultiBulksReply(bulks)
}

// 操作原始内容
func (server *GoRedisServer) OnRAW_GET(cmd *Command) (reply *Reply) {
	key, _ := cmd.ArgAtIndex(1)
	value := server.levelRedis.RawGet(key)
	if value == nil {
		reply = BulkReply(nil)
	} else {
		reply = BulkReply(value)
	}
	return
}

// 操作原始内容 RAW_SET +[hash]name latermoon
func (server *GoRedisServer) OnRAW_SET(cmd *Command) (reply *Reply) {
	key, value := cmd.Args[1], cmd.Args[2]
	err := server.levelRedis.RawSet(key, value)
	if err != nil {
		return ErrorReply(err)
	} else {
		return StatusReply("OK")
	}
}

func (server *GoRedisServer) OnRAW_SET_NOREPLY(cmd *Command) (reply *Reply) {
	server.OnRAW_SET(cmd)
	return nil
}

/**
 * 过期时间，暂不支持
 * 1 if the timeout was set.
 * 0 if key does not exist or the timeout could not be set.
 */
func (server *GoRedisServer) OnEXPIRE(cmd *Command) (reply *Reply) {
	reply = IntegerReply(0)
	return
}

func (server *GoRedisServer) OnDEL(cmd *Command) (reply *Reply) {
	keys := cmd.Args[1:]
	n := server.levelRedis.Delete(keys...)
	reply = IntegerReply(n)
	return
}

func (server *GoRedisServer) OnTYPE(cmd *Command) (reply *Reply) {
	key, _ := cmd.ArgAtIndex(1)
	t := server.levelRedis.TypeOf(key)
	if len(t) > 0 {
		reply = StatusReply(t)
	} else {
		reply = StatusReply("none")
	}
	return
}
