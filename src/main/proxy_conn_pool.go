// Copyright (c) 2021 上海骞云信息科技有限公司. All rights reserved
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	log "github.com/sirupsen/logrus"
	"sync"
)

type Pooler interface {
	Create(pool *ConnHandlerPool) (*ConnHandler, error)
	Remove(conn *ConnHandler)
	IsActive(conn *ConnHandler) bool
}

type ConnHandlerPool struct {
	Size   int
	Pooler Pooler
	mu     sync.Mutex
	conns  []*ConnHandler
}

func (connPool *ConnHandlerPool) Init() {
	connPool.conns = make([]*ConnHandler, 0, connPool.Size)
	log.Infof("init connection pool, len %d, cap %d", len(connPool.conns), cap(connPool.conns))
}

func (connPool *ConnHandlerPool) Get() (*ConnHandler, error) {
	for {
		if len(connPool.conns) == 0 {
			conn, err := connPool.Pooler.Create(connPool)
			log.Info("create connection: ", conn, err)
			if err != nil {
				return nil, err
			}

			return conn, nil
		} else {
			conn, err := connPool.getConn()
			if conn != nil {
				return conn, err
			}
		}
	}
}

func (connPool *ConnHandlerPool) getConn() (*ConnHandler, error) {
	connPool.mu.Lock()
	defer connPool.mu.Unlock()
	if len(connPool.conns) == 0 {
		return nil, nil
	}
	conn := connPool.conns[len(connPool.conns)-1]
	connPool.conns = connPool.conns[:len(connPool.conns)-1]
	if connPool.Pooler.IsActive(conn) {
		log.Info("get connection from pool: ", conn)
		return conn, nil
	} else {
		return nil, nil
	}
}

func (connPool *ConnHandlerPool) Return(conn *ConnHandler) {
	connPool.mu.Lock()
	defer connPool.mu.Unlock()
	if len(connPool.conns) >= connPool.Size {
		log.Warn("pool is full, remove connection: ", conn)
		connPool.Pooler.Remove(conn)
	} else {
		connPool.conns = connPool.conns[:len(connPool.conns)+1]
		connPool.conns[len(connPool.conns)-1] = conn
		log.Info("return connection:", conn, ", poolsize is ", len(connPool.conns))
	}
}
