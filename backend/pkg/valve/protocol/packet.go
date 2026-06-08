package protocol

import (
	"fmt"

	"l4d2-manager-next/pkg/valve/bits"
)

// from common/protocol.h
const (
	net_NOP             = 0
	net_Disconnect      = 1
	net_File            = 2
	net_SplitScreenUser = 3
	net_Tick            = 4
	net_StringCmd       = 5
	net_SetConVar       = 6
	net_SignonState     = 7

	clc_ClientInfo         = 8
	clc_Move               = 9
	clc_VoiceData          = 10
	clc_BaselineAck        = 11
	clc_ListenEvents       = 12
	clc_RespondCvarValue   = 13
	clc_FileCRCCheck       = 14
	clc_LoadingProgress    = 15
	clc_SplitPlayerConnect = 16
	clc_CmdKeyValues       = 17

	svc_ServerInfo        = 8
	svc_SendTable         = 9
	svc_ClassInfo         = 10
	svc_SetPause          = 11
	svc_CreateStringTable = 12
	svc_UpdateStringTable = 13
	svc_VoiceInit         = 14
	svc_VoiceData         = 15
	svc_Print             = 16
	svc_Sounds            = 17
	svc_SetView           = 18
	svc_FixAngle          = 19
	svc_CrosshairAngle    = 20
	svc_BSPDecal          = 21
	svc_SplitScreen       = 22
	svc_UserMessage       = 23
	svc_EntityMessage     = 24
	svc_GameEvent         = 25
	svc_PacketEntities    = 26
	svc_TempEntities      = 27
	svc_Prefetch          = 28
	svc_Menu              = 29
	svc_GameEventList     = 30
	svc_GetCvarValue      = 31
	svc_CmdKeyValues      = 32
)

type Packet interface {
	PacketType() int

	ReadFrom(r *bits.Reader) error
}

func ReadPacket(r *bits.Reader, client bool) (Packet, error) {
	id, err := r.ReadBits(6)
	if err != nil {
		return nil, err
	}

	packet := NewPacket(int(id), client)
	if packet == nil {
		if client {
			return nil, fmt.Errorf("invalid packet id from client: %d", id)
		}

		return nil, fmt.Errorf("invalid packet id from server: %d", id)
	}

	err = packet.ReadFrom(r)

	return packet, err
}

func NewPacket(id int, client bool) Packet {
	switch id {
	case net_NOP:
		return &NET_NOP{}
	case net_Disconnect:
		return &NET_Disconnect{}
	case net_File:
		return &NET_File{}
	case net_SplitScreenUser:
		return &NET_SplitScreenUser{}
	case net_Tick:
		return &NET_Tick{}
	case net_StringCmd:
		return &NET_StringCmd{}
	case net_SetConVar:
		return &NET_SetConVar{}
	case net_SignonState:
		return &NET_SignonState{}
	}

	if client {
		switch id {
		case clc_ClientInfo:
			return &CLC_ClientInfo{}
		case clc_Move:
			return &CLC_Move{}
		case clc_VoiceData:
			return &CLC_VoiceData{}
		case clc_BaselineAck:
			return &CLC_BaselineAck{}
		case clc_ListenEvents:
			return &CLC_ListenEvents{}
		case clc_RespondCvarValue:
			return &CLC_RespondCvarValue{}
		case clc_FileCRCCheck:
			return &CLC_FileCRCCheck{}
		case clc_LoadingProgress:
			return &CLC_LoadingProgress{}
		case clc_SplitPlayerConnect:
			return &CLC_SplitPlayerConnect{}
		case clc_CmdKeyValues:
			return &CLC_CmdKeyValues{}
		}
	} else {
		switch id {
		case svc_ServerInfo:
			return &SVC_ServerInfo{}
		case svc_SendTable:
			return &SVC_SendTable{}
		case svc_ClassInfo:
			return &SVC_ClassInfo{}
		case svc_SetPause:
			return &SVC_SetPause{}
		case svc_CreateStringTable:
			return &SVC_CreateStringTable{}
		case svc_UpdateStringTable:
			return &SVC_UpdateStringTable{}
		case svc_VoiceInit:
			return &SVC_VoiceInit{}
		case svc_VoiceData:
			return &SVC_VoiceData{}
		case svc_Print:
			return &SVC_Print{}
		case svc_Sounds:
			return &SVC_Sounds{}
		case svc_SetView:
			return &SVC_SetView{}
		case svc_FixAngle:
			return &SVC_FixAngle{}
		case svc_CrosshairAngle:
			return &SVC_CrosshairAngle{}
		case svc_BSPDecal:
			return &SVC_BSPDecal{}
		case svc_SplitScreen:
			return &SVC_SplitScreen{}
		case svc_UserMessage:
			return &SVC_UserMessage{}
		case svc_EntityMessage:
			return &SVC_EntityMessage{}
		case svc_GameEvent:
			return &SVC_GameEvent{}
		case svc_PacketEntities:
			return &SVC_PacketEntities{}
		case svc_TempEntities:
			return &SVC_TempEntities{}
		case svc_Prefetch:
			return &SVC_Prefetch{}
		case svc_Menu:
			return &SVC_Menu{}
		case svc_GameEventList:
			return &SVC_GameEventList{}
		case svc_GetCvarValue:
			return &SVC_GetCvarValue{}
		case svc_CmdKeyValues:
			return &SVC_CmdKeyValues{}
		}
	}

	return nil
}

type KeyValuePair struct {
	Key, Value string
}
