package streampool

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestBeatCountAndTierTargets(t *testing.T) {
	for _, tc := range []struct{ bpm, minutes float64; want int }{
		{180, DurationShort, 270}, {180, DurationNormal, 540},
		{180, DurationLong, 900}, {180, DurationMarathon, 1800},
		{177, DurationShort, 267}, {171, DurationShort, 258}, {200, .5, 99},
	} {
		got, err := BeatCount(tc.bpm, tc.minutes)
		if err != nil || got != tc.want {
			t.Fatalf("BeatCount(%g,%g)=%d %v, want %d", tc.bpm, tc.minutes, got, err, tc.want)
		}
	}
	for _, tc := range []struct{ n int; weights [3]float64; want [3]int }{
		{300, [3]float64{1,1,1}, [3]int{100,100,100}},
		{303, [3]float64{5,3,2}, [3]int{151,91,61}},
		{301, [3]float64{1,1,1}, [3]int{101,100,100}},
		{270, [3]float64{0,1,0}, [3]int{0,270,0}},
	} {
		got, err := TierTargets(tc.n, tc.weights)
		if err != nil || got != tc.want {
			t.Fatalf("TierTargets(%d,%v)=%v %v, want %v", tc.n, tc.weights, got, err, tc.want)
		}
	}
	for _, tc := range [][2]float64{{0,1.5}, {-1,1.5}, {180,0}, {180,-1}, {math.NaN(),1}, {math.Inf(1),1}, {180,math.Inf(1)}, {1e200,1e200}, {1,.001}} {
		if _, err := BeatCount(tc[0], tc[1]); err == nil { t.Fatalf("accepted duration %v", tc) }
	}
	for _, w := range [][3]float64{{0,0,0}, {-1,1,1}, {math.NaN(),1,1}, {math.Inf(1),1,1}} {
		if _, err := TierTargets(300,w); err == nil { t.Fatalf("accepted weights %v", w) }
	}
}

func TestGeneratorDeterminismAndHardConstraints(t *testing.T) {
	pool := Build()
	idx := BuildIndex(pool)
	configs := []GeneratorConfig{}
	for _, seed := range []int64{0,1,2,3,42,20261109} {
		cfg := DefaultGeneratorConfig(); cfg.Seed = seed
		configs = append(configs,cfg)
	}
	long := DefaultGeneratorConfig()
	long.Seed = 72; long.TargetMinutes = DurationMarathon
	configs = append(configs,long)
	weighted := DefaultGeneratorConfig()
	weighted.Seed = 100; weighted.TargetMinutes = DurationNormal
	weighted.TierWeights = [3]float64{5,3,2}
	configs = append(configs,weighted)
	for _, cfg := range configs {
		seq, err := Generate(pool,idx,cfg)
		if err != nil { t.Fatalf("seed=%d: %v",cfg.Seed,err) }
		repeat, err := Generate(pool,idx,cfg)
		if err != nil || !reflect.DeepEqual(seq,repeat) { t.Fatalf("not deterministic seed=%d: %v",cfg.Seed,err) }
		if len(seq.PatternIndices)!=seq.BeatCount || seq.BeatCount%3!=0 { t.Fatalf("wrong length seed=%d",cfg.Seed) }
		if seq.ActualMinutes != float64(seq.BeatCount)/cfg.BPM { t.Fatal("wrong actual minutes") }
		var actual [3]int
		var previous Mask
		window := make([]int,0,cfg.BalanceWindow)
		for beat,id := range seq.PatternIndices {
			if id<0 || id>=len(pool.All) { t.Fatalf("invalid index %d",id) }
			p:=pool.All[id]
			actual[p.DensityTier]++
			if previous&p.FirstMask!=0 { t.Fatalf("jack at beat=%d seed=%d",beat,cfg.Seed) }
			previous=p.LastMask
			window=append(window,int(p.BalanceDelta))
			if len(window)>cfg.BalanceWindow { window=window[1:] }
			sum:=0
			for _,d:=range window {sum+=d}
			if absInt(sum)>cfg.MaxBalance { t.Fatalf("hand balance=%d beat=%d seed=%d",sum,beat,cfg.Seed) }
		}
		if actual != seq.TierCounts || actual != seq.TierTargets {
			t.Errorf("seed=%d actual=%v targets=%v reported=%v",cfg.Seed,actual,seq.TierTargets,seq.TierCounts)
		}
	}
}

func TestTierAvailabilityAndFallback(t *testing.T) {
	r:=rand.New(rand.NewSource(10))
	for i:=0;i<100;i++ {
		tier,ok:=drawTier(r,[3]int{100,1,1},[3]bool{false,true,false})
		if !ok||tier!=1 {t.Fatal("unavailable tier was drawn")}
		tier,ok=drawTier(r,[3]int{0,100,100},[3]bool{true,false,false})
		if !ok||tier!=0 {t.Fatal("fallback tier was not drawn")}
		_,ok=drawTier(r,[3]int{1,1,1},[3]bool{})
		if ok {t.Fatal("accepted no legal tiers")}
	}
}

func TestGenerateBadConfig(t *testing.T) {
	pool:=Build()
	index:=BuildIndex(pool)
	cfg:=DefaultGeneratorConfig()
	cfg.BalanceWindow=0
	if _,err:=Generate(pool,index,cfg);err==nil {t.Fatal("accepted window zero")}
	cfg=DefaultGeneratorConfig()
	cfg.MaxBalance=-1
	if _,err:=Generate(pool,index,cfg);err==nil {t.Fatal("accepted negative balance cap")}
	cfg=DefaultGeneratorConfig()
	cfg.TierWeights=[3]float64{}
	if _,err:=Generate(pool,index,cfg);err==nil {t.Fatal("accepted zero tier weights")}
	cfg=DefaultGeneratorConfig()
	if _,err:=Generate(Pool{},Index{},cfg);err==nil {t.Fatal("accepted empty pool")}
}
