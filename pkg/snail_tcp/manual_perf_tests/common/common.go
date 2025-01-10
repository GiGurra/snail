package common

import "github.com/GiGurra/snail/pkg/snail_tcp"

var Port = 12345
var NClients = 32
var WriteBufSize = 10 * 1024 * 1024
var ReadBufSize = WriteBufSize
var TcpWindowSize = 10 * 1024 * 1024
var Optimization = snail_tcp.OptimizeForThroughput
