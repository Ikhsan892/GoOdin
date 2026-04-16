# Uber FX & Dependency Injection di GoOdin — Penjelasan Santai

Dokumen ini jelasin:

1. Apa itu Uber FX dan kenapa kepake di project ini
2. Cara kerjanya: `fx.Provide`, `fx.Invoke`, `fx.In`, `fx.Out`
3. Kenapa keliatan "ajaib" — sebenernya apa yang terjadi di balik layar
4. Refactor `orderDetailReq` & kawan-kawan jadi sub-module FX (nested DI)

---

## 1. Problem-nya dulu: kenapa kita butuh DI container

Coba bayangin tanpa DI container. Di `main.go` lu bakal nulis:

```go
db := openDB(config)
natsInfra := newNats(app)
writeRepo := repositories.NewLoyaltyWriteGormRepository(db)
idemRepo := repositories.NewIdempotencyGormRepository(db)
eventBus := newEventBus(router, logger)
orderDetailReq := NewOrderDetailRequestImpl(app, natsInfra)
creditHandler := command.NewCreditLoyaltyHandler(app, writeRepo, idemRepo, eventBus, orderDetailReq)
// ... 50 baris lagi kayak gini buat domain lain
```

Masalahnya:

- **Urutan penting**: `db` harus dibuat duluan, baru repo, baru handler. Kalo salah urut, compile OK tapi runtime panic.
- **Ribet kalo banyak domain**: tiap nambah 1 handler baru, lu harus bongkar `main.go`.
- **Susah di-test**: mau ganti `db` pake mock? Harus ubah `main.go`.
- **Gampang salah**: lupa construct 1 thing → nil pointer.

DI container solusinya: **lu kasih tau container "gw butuh X, gw bisa bikin Y dari X", terus container yang urusin urutan + construct semuanya.**

---

## 2. Konsep inti Uber FX

FX itu DI container buat Go. Intinya cuma 3 hal:

### 2.1 `fx.Provide` — "Gw bisa bikin X kalo dikasih A, B, C"

```go
fx.Provide(
    func(db *gorm.DB) repository.LoyaltyWriteRepository {
        return repositories.NewLoyaltyWriteGormRepository(db)
    },
)
```

Baca kayak resep: "Buat bikin `LoyaltyWriteRepository`, gw butuh `*gorm.DB`. Kalo lu kasih `*gorm.DB`, gw balikin `LoyaltyWriteRepository`."

FX bakal simpen resep ini di graph, tapi **belum dijalanin**. Fungsinya baru dipanggil kalo ada yang minta `LoyaltyWriteRepository`.

### 2.2 `fx.Invoke` — "Jalanin fungsi ini, sekarang juga"

```go
fx.Invoke(func(e *echo.Echo) {
    // e udah siap, graph udah di-build
})
```

`Invoke` = titik masuknya. Dia yang "mulai" program. Begitu ada `Invoke`, FX ngeliat: "Oh fungsi ini butuh `*echo.Echo`. `*echo.Echo` siapa yang bikin? Oh `NewEchoFX`. `NewEchoFX` butuh apa? Dst..." → **FX traversal graph, jalanin Provide yang perlu aja, urut dari yang paling bawah**.

Kalo ga ada `Invoke`, program ga jalan — semua `Provide` cuma nganggur di graph.

### 2.3 `fx.Module` — "Paket Provide yang related"

```go
var Module = fx.Module("echo_http",
    fx.Provide(NewEchoFX, AsEchoRoute(api.NewLoyaltyController)),
)
```

Module cuma cara ngerapihin Provide biar `cmd/all.go` ga keliatan kayak neraka. Nanti dipanggil:

```go
fx.New(
    watermillfx.Module,
    httpdriver.Module,
    fx.Invoke(func(*echo.Echo) {}),
).Run()
```

---

## 3. `fx.In` dan `fx.Out` — dua trik yang bikin FX keliatan keren

### 3.1 `fx.In` — "Gw minta banyak dependency, tolong di-inject ke struct gw"

Tanpa `fx.In`, constructor lu cuma bisa nerima parameter biasa:

```go
func NewRegistrar(app core.App, db *gorm.DB) *Registrar { ... }
```

Kalo param-nya ada 8, constructor-nya jadi panjang dan urutan penting. Solusinya: bikin struct yang embed `fx.In`:

```go
type Params struct {
    fx.In                          // <-- marker, bilang ke FX: "struct ini dipake buat inject"

    App core.App
    DB  *gorm.DB `name:"gorm"`     // <-- minta yg named "gorm"
}

func NewRegistrar(p Params) *Registrar {
    return &Registrar{app: p.App, db: p.DB}
}
```

FX liat struct embed `fx.In` → "Oh ini minta dependency per-field. Gw inject satu per satu." Urutan field ga penting, tinggal nambah field baru juga gampang.

Tag `name:"gorm"` itu karena bisa ada **banyak `*gorm.DB`** di graph (misalnya koneksi main + analytics). Liat `datamanagerfx/datamanager.go:50-62` — `ProvideGorm("gorm")` dan `ProvideGorm("analytics")` bikin 2 `*gorm.DB` yang beda, dibedain pake nama.

### 3.2 `fx.Out` — "Gw kasih banyak dependency sekaligus"

Kebalikan dari `fx.In`. Kalo constructor lu mau balikin **beberapa hasil** yang berbeda:

```go
type Buses struct {
    fx.Out                         // <-- marker, bilang ke FX: "struct ini buat output"

    CommandBus cqrs.CommandBus
    QueryBus   cqrs.QueryBus
    EventBus   cqrs.EventBus
}

func New(p Params) (Buses, error) {
    // ... bikin semua bus ...
    return Buses{
        CommandBus: cmdBus,
        QueryBus:   qBus,
        EventBus:   evtBus,
    }, nil
}
```

FX liat return value embed `fx.Out` → "Oh ini 3 dependency sekaligus: `CommandBus`, `QueryBus`, `EventBus`. Gw daftarin semuanya di graph." Liat `drivers/watermill_fx/watermill.go:44-91`.

### 3.3 Value groups — "Kumpulin semua impl interface ini"

Nah ini yang bener-bener keren. Kalo lu punya banyak handler/controller yang implement interface yang sama, lu bisa minta FX kumpulin **semuanya** jadi slice:

```go
// Producer side: tiap domain daftarin handler-nya
func AsHandler(f any) any {
    return fx.Annotate(f,
        fx.As(new(cqrs.HandlerRegistrar)),
        fx.ResultTags(`group:"watermill_handlers"`),  // <-- masuk group ini
    )
}

// Consumer side: minta semua handler dalam group
type Params struct {
    fx.In

    Registrars []cqrs.HandlerRegistrar `group:"watermill_handlers"`  // <-- FX isi otomatis
}
```

Hasilnya: tiap nambah domain baru (misalnya `payments`), tinggal tambahin 1 baris `AsHandler(payments.NewRegistrar)` di module — ga perlu sentuh consumer. Itu sebabnya `watermillfx/watermill.go:96-101` cuma 5 baris tapi scalable.

---

## 4. Walkthrough: gimana `cmd/all.go` bangun aplikasi

Pas lu jalanin `make run/dev`, urutannya:

```
1. cmd/all.go: app.OnAfterApplicationBootstrapped (load config, init DataManager)
2. cmd/all.go: RunDriver(monitoring) kalo enabled
3. cmd/all.go: RunDriver(NATS)                          ← NATS driver non-FX, dijalanin duluan
4. cmd/all.go: fx.New(...)                              ← baru FX mulai
   ├─ Register semua Provide ke graph (BELUM dijalanin)
   ├─ Ketemu fx.Invoke(func(*echo.Echo){})
   ├─ Traversal graph:
   │   *echo.Echo → NewEchoFX → butuh []EchoRoute (group) + fx.Lifecycle
   │   []EchoRoute → kumpulin semua AsEchoRoute(...)
   │   LoyaltyController → butuh QueryBus → dari watermillfx.New
   │   watermillfx.New → butuh []HandlerRegistrar (group) + fx.Lifecycle + *slog.Logger
   │   []HandlerRegistrar → kumpulin semua AsHandler(...)
   │   loyalty.NewRegistrar → butuh App + *gorm.DB(name:"gorm")
   │   *gorm.DB(name:"gorm") → ProvideGorm("gorm") → butuh *datamanager.DataManager
   │   *datamanager.DataManager → NewDataManager(app) → langsung balikin
   │
   └─ FX jalanin dari daun ke akar:
      DataManager → *gorm.DB → Registrar → HandlerRegistrars slice
      → watermillfx.New (register handlers ke bus, attach lifecycle hooks)
      → LoyaltyController → []EchoRoute → NewEchoFX → *echo.Echo
      → Invoke callback jalan → lifecycle OnStart dipanggil → server nyala
```

Yang bikin FX keliatan ajaib = **topological sort otomatis**. Lu ga perlu mikirin "siapa harus dibikin duluan". FX yang mikirin, karena dia tau siapa butuh siapa dari signature function.

---

## 5. `fx.Lifecycle` — start/stop hook

Service kayak Echo atau Watermill router perlu hidup selama aplikasi jalan. FX kasih `fx.Lifecycle`:

```go
func New(p Params) Buses {
    // ... construct ...
    p.LC.Append(fx.Hook{
        OnStart: func(ctx context.Context) error { return router.Run(ctx) },
        OnStop:  func(ctx context.Context) error { return router.Close() },
    })
    return Buses{...}
}
```

FX yang urusin kapan manggil `OnStart` (habis graph dibangun) dan `OnStop` (pas SIGTERM). Lu cuma daftarin hook-nya.

---

## 6. Kondisi sekarang di `use_cases/loyalty/registrar.go`

Sekarang `Registrar` itu "composition root" manual buat loyalty — semua construct-annya dikerjain di method `Register()`. Dependency yang di-inject via `fx.In` cuma **App + DB**, sisanya construct manual:

```go
// loyalty/registrar.go (existing)
type Params struct {
    fx.In
    App core.App
    DB  *gorm.DB `name:"gorm"`
}

func (r *Registrar) Register(cmd cqrs.RegisterableCommandBus, qry cqrs.RegisterableQueryBus, evt cqrs.EventBus) {
    writeRepo := repositories.NewLoyaltyWriteGormRepository(r.db)    // ← manual
    readRepo := repositories.NewLoyaltyReadGormRepository(r.db)       // ← manual
    idemRepo := repositories.NewIdempotencyGormRepository(r.db)       // ← manual
    // ...
    var orderDetailReq request.OrderDetailRequest
    if natsDrv := r.app.Driver().Instance(nats_messaging.NATS_DRIVER); natsDrv != nil {  // ← manual + ribet
        if infra, ok := natsDrv.(nats_infra.NatsInfrastructure); ok {
            orderDetailReq = NewOrderDetailRequestImpl(r.app, infra)
        }
    }
    creditHandler := command.NewCreditLoyaltyHandler(r.app, writeRepo, idemRepo, evt)  // ← manual
}
```

Ini hybrid: FX-dependent di luar, DI manual di dalem. Masalahnya:

- NATS di-resolve via `app.Driver().Instance()` + type assertion — bau manual lookup
- Kalo ada handler baru yang butuh `OrderDetailRequest`, harus tambah parameter manual terus-terusan
- `_ = orderDetailReq` cuma placeholder — beneran ga kepake karena handler belom di-refactor

**Goal refactor: pindah semua construct ke FX Provide, nested di dalam sub-module `loyaltyfx`.**

---

## 7. Refactor: sub-module FX untuk Loyalty (nested DI)

Ide: bikin **module FX baru** `loyaltyfx.Module` yang:

1. `fx.Provide` semua repo, ACL request, dan handler-nya
2. `fx.Provide` sebuah `*Registrar` baru yang cuma nerima handler (bukan construct-nya)
3. Di-Include di `watermillfx.Module` via `AsHandler(loyaltyfx.NewRegistrar)`
4. NATS infra juga di-Provide ke FX graph biar bisa di-inject

### 7.1 Bikin NATS sebagai FX value

Bikin file baru `drivers/messaging/nats/fx.go`:

```go
package nats_messaging

import (
    nats_infra "goodin/infrastructure/messaging/nats"
    core "goodin/internal"
    "go.uber.org/fx"
)

// ProvideNats extracts the NATS infra from the app.Driver() registry
// so it can be injected into any FX consumer.
// Requires app.Driver().RunDriver(NewNatsMessaging(app, true)) to run before FX.
func ProvideNats(app core.App) nats_infra.NatsInfrastructure {
    drv := app.Driver().Instance(NATS_DRIVER)
    if drv == nil {
        return nil
    }
    infra, _ := drv.(nats_infra.NatsInfrastructure)
    return infra
}

var Module = fx.Module("nats",
    fx.Provide(ProvideNats),
)
```

Lalu di `cmd/all.go` tambah `nats_messaging.Module` di `fx.New(...)`. Sekarang **siapapun di graph bisa minta `nats_infra.NatsInfrastructure`** lewat `fx.In` — ga perlu lagi `app.Driver().Instance(...)` manual.

### 7.2 Pecah Loyalty jadi sub-module `loyaltyfx`

Bikin file baru `use_cases/loyalty/fx.go`:

```go
package loyalty

import (
    nats_infra "goodin/infrastructure/messaging/nats"
    "goodin/pkg/cqrs"
    "goodin/repositories"
    "goodin/use_cases/loyalty/command"
    loyaltyevent "goodin/use_cases/loyalty/event"
    "goodin/use_cases/loyalty/port/input/message/request"
    "goodin/use_cases/loyalty/port/output/idempotency"
    "goodin/use_cases/loyalty/port/output/repository"
    "goodin/use_cases/loyalty/query"

    core "goodin/internal"
    "go.uber.org/fx"
    "gorm.io/gorm"
)

// ── Repos ───────────────────────────────────────────────────────────────────
func provideWriteRepo(p struct {
    fx.In
    DB *gorm.DB `name:"gorm"`
}) repository.LoyaltyWriteRepository {
    return repositories.NewLoyaltyWriteGormRepository(p.DB)
}

func provideReadRepo(p struct {
    fx.In
    DB *gorm.DB `name:"gorm"`
}) repository.LoyaltyReadRepository {
    return repositories.NewLoyaltyReadGormRepository(p.DB)
}

func provideIdemRepo(p struct {
    fx.In
    DB *gorm.DB `name:"gorm"`
}) idempotency.IdempotencyRepository {
    return repositories.NewIdempotencyGormRepository(p.DB)
}

// ── ACL ─────────────────────────────────────────────────────────────────────
func provideOrderDetailRequest(app core.App, infra nats_infra.NatsInfrastructure) request.OrderDetailRequest {
    if infra == nil {
        app.Logger().Warn("Loyalty ACL unavailable: NATS infra nil")
        return nil
    }
    return NewOrderDetailRequestImpl(app, infra)
}

// ── Handlers ────────────────────────────────────────────────────────────────
func provideCreditHandler(
    app core.App,
    w repository.LoyaltyWriteRepository,
    idem idempotency.IdempotencyRepository,
    evt cqrs.EventBus,
    orderReq request.OrderDetailRequest, // ← di-inject otomatis
) *command.CreditLoyaltyHandler {
    return command.NewCreditLoyaltyHandler(app, w, idem, evt /*, orderReq kalo udah dipake */)
}

func provideBalanceHandler(app core.App, r repository.LoyaltyReadRepository) *query.GetLoyaltyBalanceHandler {
    cache := repositories.NewNoopLoyaltyCacheRepository()
    return query.NewGetLoyaltyBalanceHandler(app, r, cache)
}

func provideOnOrderHandler(app core.App, cmd cqrs.CommandBus, evt cqrs.EventBus) *loyaltyevent.OnOrderCompletedHandler {
    return loyaltyevent.NewOnOrderCompletedHandler(app, cmd, evt)
}

// ── Module ──────────────────────────────────────────────────────────────────
// Module sub-FX buat domain loyalty. Semua construct via fx.Provide,
// handler di-inject otomatis ke Registrar.
var Module = fx.Module("loyalty",
    fx.Provide(
        provideWriteRepo,
        provideReadRepo,
        provideIdemRepo,
        provideOrderDetailRequest,
        provideCreditHandler,
        provideBalanceHandler,
        provideOnOrderHandler,
    ),
)
```

### 7.3 Ubah `Registrar` jadi tipis — cuma wire ke bus

```go
// loyalty/registrar.go (setelah refactor)

type RegistrarParams struct {
    fx.In

    App             core.App
    CreditHandler   *command.CreditLoyaltyHandler
    BalanceHandler  *query.GetLoyaltyBalanceHandler
    OnOrderHandler  *loyaltyevent.OnOrderCompletedHandler
}

type Registrar struct {
    p RegistrarParams
}

func NewRegistrar(p RegistrarParams) *Registrar { return &Registrar{p: p} }

func (r *Registrar) Register(cmd cqrs.RegisterableCommandBus, qry cqrs.RegisterableQueryBus, evt cqrs.EventBus) {
    cmd.RegisterHandler("loyalty.credit_loyalty", func(ctx context.Context, c cqrs.Command) (any, error) {
        return r.p.CreditHandler.Handle(ctx, c.(dto.CreditLoyaltyRequest))
    })
    qry.RegisterHandler("loyalty.get_balance", func(ctx context.Context, q cqrs.Query) (any, error) {
        return r.p.BalanceHandler.Handle(ctx, q.(dto.GetLoyaltyBalanceRequest))
    })
    evt.Subscribe(pkgevents.OrderCompleted, r.p.OnOrderHandler)
}
```

Jadi tinggal: handler udah di-construct sama FX (lewat `loyalty.Module`), Registrar tinggal colok ke bus.

### 7.4 Gabungin di root

`drivers/watermill_fx/watermill.go` (udah ada `AsHandler(loyalty.NewRegistrar)` — ga berubah).

`cmd/all.go`:

```go
fx.New(
    // ... existing ...
    datamanagerfx.Module,          // kalo udah dipisah jadi module
    nats_messaging.Module,         // ← BARU
    loyalty.Module,                // ← BARU (sub-FX domain)
    watermillfx.Module,
    httpdriver.Module,
    fx.Invoke(func(*echo.Echo) {}),
).Run()
```

### 7.5 "Nested DI" yang lu tanyain

Sekarang struktur graph-nya jadi nested beneran:

```
cmd/all.go fx.New
├── datamanagerfx.Module           Provide *gorm.DB (name:"gorm")
├── nats_messaging.Module          Provide nats_infra.NatsInfrastructure
├── loyalty.Module                  ← sub-module, DI di dalem
│   ├── Provide LoyaltyWriteRepository    (minta *gorm.DB)
│   ├── Provide LoyaltyReadRepository     (minta *gorm.DB)
│   ├── Provide IdempotencyRepository     (minta *gorm.DB)
│   ├── Provide OrderDetailRequest        (minta NatsInfrastructure)
│   ├── Provide CreditLoyaltyHandler      (minta 5 dep)
│   ├── Provide GetLoyaltyBalanceHandler  (minta 2 dep)
│   └── Provide OnOrderCompletedHandler   (minta 3 dep)
├── watermillfx.Module              Consume Registrar (dari loyalty.Module)
└── httpdriver.Module               Consume LoyaltyController
```

Tiap module isolated — kalo lu mau test `CreditLoyaltyHandler` di isolasi, tinggal `fx.New(loyalty.Module, ...mocks)` tanpa bawa Echo/Watermill/NATS. Itu nilai jual utama nested module.

---

## 8. Cheat sheet

| Konsep | Fungsi | Contoh di repo |
|---|---|---|
| `fx.Provide(f)` | Daftarin "resep" — constructor | `datamanagerfx.NewDataManager` |
| `fx.Invoke(f)` | Jalanin fungsi = titik masuk program | `cmd/all.go:60` |
| `fx.In` | Marker struct = "inject ke field-field gw" | `watermillfx.Params` |
| `fx.Out` | Marker struct = "gw balikin banyak value" | `watermillfx.Buses` |
| `fx.Annotate` | Kasih metadata (name, group, as-interface) | `datamanagerfx.ProvideGorm` |
| `name:"x"` tag | Bedain 2 value dengan tipe sama | `*gorm.DB` gorm vs analytics |
| `group:"x"` tag | Kumpulin banyak impl jadi slice | `"watermill_handlers"` |
| `fx.Lifecycle` | Hook buat OnStart/OnStop | `watermillfx.New:71` |
| `fx.Module(name, ...)` | Bundle Provide jadi reusable unit | `watermillfx.Module` |

---

## 9. TL;DR

- FX = DI container. Lu kasih resep (`Provide`), FX urusin urutan dan eksekusi.
- `fx.In` = "struct gw mau di-inject field-field-nya"
- `fx.Out` = "struct gw mau balikin banyak value sekaligus"
- Cara keren-nya cuma 2 trik: **topological sort otomatis** + **value groups**.
- Nested module = satu aplikasi = banyak `fx.Module` yang include-include. Tiap domain bisa punya module-nya sendiri.
- Refactor `orderDetailReq` = pindah dari manual construct ke `fx.Provide`, supaya handler bisa minta via constructor biasa. Ga ada lagi `app.Driver().Instance(...)` manual lookup.
