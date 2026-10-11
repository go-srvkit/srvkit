// Copyright © 2023-2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"time"

	"gopkg.in/srvkit.v1/logger"
)

const WriteBufferReuseSize = 1 << 12

var (
	TCPReadTimeout = 300 * time.Second // 默认读超时
	verbose        bool
)

// TcpSession TCP连接会话
type TcpSession struct {
	StreamBase

	conn       net.Conn       // tcp Conn
	wg         sync.WaitGroup // sync flush write
	done       chan struct{}  // sync write pump
	encrypt    IEncryptor     // 加解密
	decrypt    IEncryptor     //
	codec      ICodec         //
	recvHead   []byte         // recv head buffer
	intranet   bool           // 内联网
	signCRC    bool           // 发包是否增加CRC校验
	decodeBody bool           // 解码消息体
}

var _ Endpoint = (*TcpSession)(nil)

func NewTcpSession(conn net.Conn, intranet, decodeBody bool, sendQSize int, recvQ chan *NetMessage, errQ chan *Error) *TcpSession {
	return new(TcpSession).Init(conn, intranet, decodeBody, sendQSize, recvQ, errQ)
}

func (t *TcpSession) Init(conn net.Conn, intranet, decodeBody bool, sendQSize int, recvQ chan *NetMessage, errQ chan *Error) *TcpSession {
	t.StreamBase.Init(sendQSize, recvQ, errQ)
	t.StreamBase.remoteAddr = conn.RemoteAddr().String()
	t.conn = conn
	t.done = make(chan struct{})
	t.intranet = intranet
	t.decodeBody = decodeBody
	if intranet {
		t.codec = new(S2SCodec)
		t.recvHead = make([]byte, S2SHeaderLength)
	} else {
		t.codec = new(C2SCodec)
		t.recvHead = make([]byte, C2SHeaderLength)
	}
	return t
}

func (t *TcpSession) UnderlyingConn() net.Conn {
	return t.conn
}

func (t *TcpSession) SetIntranet(v bool) {
	t.intranet = v
}

func (t *TcpSession) SetEncryption(enc, dec IEncryptor) {
	t.encrypt = enc
	t.decrypt = dec
}

func (t *TcpSession) Go(ctx context.Context, reader, writer bool) {
	if writer {
		t.wg.Add(1)
		go t.writePump(ctx)
	}
	if reader {
		t.wg.Add(1)
		go t.readPump(ctx)
	}
}

func (t *TcpSession) Close() error {
	if !t.SetClosing() {
		return nil // close in progress
	}
	if conn, ok := t.conn.(*net.TCPConn); ok {
		if err := conn.CloseRead(); err != nil {
			logger.Infof("%v close read: %v", t.node, err)
		}
	}
	t.finally(ErrConnForceClose) // 阻塞等待投递剩余的消息
	return nil
}

func (t *TcpSession) ForceClose(reason error) {
	if !t.SetClosing() {
		return // close in progress
	}
	if conn, ok := t.conn.(*net.TCPConn); ok {
		if err := conn.CloseRead(); err != nil {
			logger.Infof("%v close read: %v", t.node, err)
		}
	}
	go t.finally(reason) // 不阻塞等待
}

func (t *TcpSession) finally(reason error) {
	close(t.done) // 通知发送线程flush并退出
	t.AppendErr(reason, t)
	t.wg.Wait()

	t.conn = nil
	t.StreamBase.Reset()
}

func (t *TcpSession) flush() {
	defer t.conn.Close() // close after flush, and this stops reader pump

	var buf bytes.Buffer
	t.conn.SetWriteDeadline(time.Now().Add(time.Minute))
	for {
		select {
		case netMsg, ok := <-t.sendQ:
			if !ok {
				return
			}
			buf.Reset()
			if err := t.write(netMsg, &buf); err != nil {
				logger.Errorf("%v flush message %v: %v", t.node, netMsg.MsgId, err)
			}
		default:
			return
		}
	}
}

func (t *TcpSession) write(netMsg *NetMessage, buf *bytes.Buffer) error {
	if verbose {
		logger.Debugf("TcpSession: node %v write messge %d %T", t.node, netMsg.MsgId, netMsg.Body)
	}
	if err := t.codec.EncodeNetMsg(netMsg, t.encrypt, t.signCRC, buf); err != nil {
		return err
	}
	if _, err := t.conn.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

func (t *TcpSession) writePump(ctx context.Context) {
	defer func() {
		t.flush()
		t.wg.Done()
		logger.Debugf("TcpSession: node %v writer stopped", t.node)
	}()

	logger.Debugf("TcpSession: node %v(%v) writer started", t.node, t.remoteAddr)
	var buf = new(bytes.Buffer)
	for {
		select {
		case netMsg, ok := <-t.sendQ:
			if !ok {
				return
			}
			// reuse small size buffer
			if buf.Cap() < WriteBufferReuseSize {
				buf.Reset()
			} else {
				buf = new(bytes.Buffer)
			}
			if err := t.write(netMsg, buf); err != nil {
				logger.Errorf("%v write message %d: %v", t.node, netMsg.MsgId, err)
			}
			FreeNetMessage(netMsg)

		case <-t.done:
			return

		case <-ctx.Done():
			return
		}
	}
}

func (t *TcpSession) ReadMessage(rd io.Reader, netMsg *NetMessage) error {
	var maxSize = C2SMaxUpStreamSize
	if t.intranet {
		maxSize = MaxS2SPacketSize
	}
	var deadline = time.Now().Add(TCPReadTimeout)
	if err := t.conn.SetReadDeadline(deadline); err != nil {
		logger.Errorf("session %v set read deadline: %v", t.node, err)
	}
	for i := 0; i < len(t.recvHead); i++ {
		t.recvHead[i] = 0 // reset header buffer
	}
	body, err := t.codec.ReadHeadBody(rd, t.recvHead, uint32(maxSize))
	if err != nil {
		return err
	}
	if err = t.codec.DecodeMsgTo(t.recvHead, body, t.decrypt, netMsg); err != nil {
		return err
	}
	if t.decodeBody {
		if err = netMsg.AutoDecode(); err != nil {
			return err
		}
	}
	netMsg.CreatedAt = time.Now().UnixMicro()
	netMsg.Session = t
	if verbose {
		logger.Debugf("TcpSession: node %v recv messge %d %T", t.node, netMsg.MsgId, netMsg.Body)
	}
	return nil
}

func (t *TcpSession) readPump(ctx context.Context) {
	defer func() {
		t.wg.Done()
		logger.Debugf("TcpSession: node %v reader stopped", t.node)
	}()
	if verbose {
		logger.Debugf("TcpSession: node %v(%v) reader started", t.node, t.remoteAddr)
	}
	var rd = bufio.NewReader(t.conn)
	for {
		var netMsg = AllocNetMessage()
		if err := t.ReadMessage(rd, netMsg); err != nil {
			if err != io.EOF {
				logger.Errorf("session %v read packet %v", t.node, err)
			}
			t.ForceClose(err) // I/O超时或者发生错误，强制关闭连接
			return
		}
		t.recvQ <- netMsg // 如果channel满了，不能丢弃，需要阻塞等待

		if t.IsClosing() {
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
