package bsp

type Contents uint32

const (
	ContentsEmpty              Contents = 0
	ContentsSolid              Contents = 0x1
	ContentsWindow             Contents = 0x2
	ContentsAux                Contents = 0x4
	ContentsGrate              Contents = 0x8
	ContentsSlime              Contents = 0x10
	ContentsWater              Contents = 0x20
	ContentsBlockLOS           Contents = 0x40
	ContentsOpaque             Contents = 0x80
	ContentsTestFogVolume      Contents = 0x100
	ContentsTeam1              Contents = 0x800
	ContentsTeam2              Contents = 0x1000
	ContentsIgnoreNoDrawOpaque Contents = 0x2000
	ContentsMoveable           Contents = 0x4000
	ContentsAreaPortal         Contents = 0x8000
	ContentsPlayerClip         Contents = 0x10000
	ContentsMonsterClip        Contents = 0x20000
	ContentsCurrent0           Contents = 0x40000
	ContentsCurrent90          Contents = 0x80000
	ContentsCurrent180         Contents = 0x100000
	ContentsCurrent270         Contents = 0x200000
	ContentsCurrentUp          Contents = 0x400000
	ContentsCurrentDown        Contents = 0x800000
	ContentsOrigin             Contents = 0x1000000
	ContentsMonster            Contents = 0x2000000
	ContentsDebris             Contents = 0x4000000
	ContentsDetail             Contents = 0x8000000
	ContentsTransluscent       Contents = 0x10000000
	ContentsLadder             Contents = 0x20000000
	ContentsHitbox             Contents = 0x40000000
)

const (
	MaskAll                  = ^Contents(0)
	MaskSolid                = ContentsSolid | ContentsMoveable | ContentsWindow | ContentsMonster | ContentsGrate
	MaskPlayerSolid          = MaskSolid | ContentsPlayerClip
	MaskNPCSolid             = MaskSolid | ContentsMonsterClip
	MaskNPCFluid             = MaskNPCSolid &^ ContentsGrate
	MaskWater                = ContentsWater | ContentsMoveable | ContentsSlime
	MaskOpaque               = ContentsSolid | ContentsMoveable | ContentsOpaque
	MaskOpaqueAndNPCs        = MaskOpaque | ContentsMonster
	MaskBlockLOS             = ContentsSolid | ContentsMoveable | ContentsBlockLOS
	MaskBlockLOSAndNPCs      = MaskBlockLOS | ContentsMonster
	MaskVisible              = MaskOpaque | ContentsIgnoreNoDrawOpaque
	MaskVisibleAndNPCs       = MaskOpaqueAndNPCs | ContentsIgnoreNoDrawOpaque
	MaskShot                 = ContentsSolid | ContentsMoveable | ContentsMonster | ContentsWindow | ContentsDebris | ContentsHitbox
	MaskShotBrushOnly        = ContentsSolid | ContentsMoveable | ContentsWindow | ContentsDebris
	MaskShotHull             = ContentsSolid | ContentsMoveable | ContentsMonster | ContentsWindow | ContentsDebris | ContentsGrate
	MaskShotPortal           = ContentsSolid | ContentsMoveable | ContentsWindow | ContentsMonster
	MaskSolidBrushOnly       = ContentsSolid | ContentsMoveable | ContentsWindow | ContentsGrate
	MaskPlayerSolidBrushOnly = MaskSolidBrushOnly | ContentsPlayerClip
	MaskNPCSolidBrushOnly    = MaskSolidBrushOnly | ContentsMonsterClip
	MaskNPCWorldStatic       = ContentsSolid | ContentsWindow | ContentsMonsterClip | ContentsGrate
	MaskNPCWorldStaticFluid  = ContentsSolid | ContentsWindow | ContentsMonsterClip
	MaskSplitAreaPortal      = ContentsWater | ContentsSlime
)

type Surf uint16

const (
	SurfLight     Surf = 0x0001
	SurfSky2D     Surf = 0x0002
	SurfSky       Surf = 0x0004
	SurfWarp      Surf = 0x0008
	SurfTrans     Surf = 0x0010
	SurfNoPortal  Surf = 0x0020
	SurfTrigger   Surf = 0x0040
	SurfNoDraw    Surf = 0x0080
	SurfHint      Surf = 0x0100
	SurfSkip      Surf = 0x0200
	SurfNoLight   Surf = 0x0400
	SurfBumpLight Surf = 0x0800
	SurfNoShadows Surf = 0x1000
	SurfNoDecals  Surf = 0x2000
	SurfNoChop    Surf = 0x4000
	SurfHitbox    Surf = 0x8000
)

type Solid uint8

const (
	SolidNone     Solid = 0
	SolidBSP      Solid = 1
	SolidBBox     Solid = 2
	SolidVPhysics Solid = 6
)
