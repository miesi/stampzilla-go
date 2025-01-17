package main

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newPrice(t1 time.Time, level string) Price {
	return Price{Time: t1, Level: level}
}

func newPriceTotal(hour int, total float64) Price {
	t1 := time.Date(2020, 10, 10, hour, 0, 0, 0, time.UTC)
	return Price{Time: t1.Truncate(time.Hour), Total: total}
}

func TestClearOld(t *testing.T) {
	t.Parallel()
	price1 := newPrice(time.Now(), "")
	price2 := newPrice(time.Now().Add(-24*time.Hour), "")

	prices := NewPrices()
	prices.Add(price1)
	prices.Add(price2)

	assert.Len(t, prices.prices, 2)
	prices.ClearOld()
	assert.Len(t, prices.prices, 1)
}

func TestCurrent(t *testing.T) {
	t.Parallel()
	price1 := newPrice(time.Now(), "now")
	price2 := newPrice(time.Now().Add(-1*time.Hour), "lasthour")
	price3 := newPrice(time.Now().Add(1*time.Hour), "nexthour")

	prices := NewPrices()
	prices.Add(price1)
	prices.Add(price2)
	prices.Add(price3)

	cur := prices.Current()
	assert.Equal(t, "now", cur.Level)
}

func TestHasTomorrowPrices(t *testing.T) {
	t.Parallel()
	price1 := newPrice(time.Now().Truncate(24*time.Hour).Add(24*time.Hour), "")

	t.Log(price1)

	prices := NewPrices()
	prices.Add(price1)

	assert.True(t, prices.HasTomorrowPricesYet())
}

func TestCalculateBestChargeHours(t *testing.T) {
	t.Parallel()
	tests := []struct {
		hoursPrice []struct {
			hour  int
			price float64
		}
		exp int
	}{
		{
			hoursPrice: []struct {
				hour  int
				price float64
			}{
				{0, 0},
				{1, 10},
				{2, 0},
				{3, 1},
				{4, 10},
				{5, 0},
				{6, 0},
				{7, 0},
				{8, 0},
				{9, 5},
				{10, 5},
				{11, 5},
				{12, 0},
				{13, 0},
				{14, 0},
				{15, 0},
				{16, 0},
			},
			exp: 11,
		},
		{
			hoursPrice: []struct {
				hour  int
				price float64
			}{
				{0, 0},
				{1, 0},
				{2, 0},
				{3, 0},
				{4, 0},
				{5, 1},
				{6, 1},
				{7, 1},
				{8, 1},
				{9, 1},
				{10, 1},
				{11, 1},
				{12, 1},
				{13, 1},
				{14, 1},
				{15, 1},
				{16, 1},
			},
			exp: 0,
		},
		{
			hoursPrice: []struct {
				hour  int
				price float64
			}{
				{0, 10},
				{1, 10},
				{2, 10},
				{3, 0},
				{4, 0},
				{5, 1},
				{6, 1},
				{7, 10},
				{8, 1},
				{9, 1},
				{10, 1},
				{11, 1},
				{12, 1},
				{13, 10},
				{14, 10},
				{15, 10},
				{16, 10},
			},
			exp: 3,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("expected %d", tt.exp), func(t *testing.T) {
			t.Parallel()
			prices := NewPrices()
			for _, v := range tt.hoursPrice {
				prices.Add(newPriceTotal(v.hour, v.price))
			}
			start := time.Date(2020, 10, 10, 0, 0, 0, 0, time.UTC)
			cheapestStartTime := prices.calculateBestChargeHours(start, 6*time.Hour)

			t.Log("cheapestStartTime", cheapestStartTime)
			expected := time.Date(2020, 10, 10, tt.exp, 0, 0, 0, time.UTC)
			assert.Equal(t, expected, cheapestStartTime)
		})
	}
}

func TestCheapestSingleHour(t *testing.T) {
	t.Parallel()
	tests := []struct {
		hoursPrice []struct {
			hour  int
			price float64
		}
		exp int
	}{

		{
			hoursPrice: []struct {
				hour  int
				price float64
			}{
				{0, 30},
				{1, 29},
				{2, 28},
				{3, 27},
				{4, 25},
				{5, 15},
				{6, 2},
				{7, 11},
				{8, 12},
				{9, 13},
				{10, 14},
				{11, 1},
				{12, 16},
				{13, 2},
				{14, 18},
				{15, 19},
				{16, 20},
			},
			exp: 11,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("expected %d", tt.exp), func(t *testing.T) {
			t.Parallel()
			prices := NewPrices()
			for _, v := range tt.hoursPrice {
				prices.Add(newPriceTotal(v.hour, v.price))
			}
			start := time.Date(2020, 10, 10, 0, 0, 0, 0, time.UTC)
			end := time.Date(2020, 10, 10, 16, 0, 0, 0, time.UTC)
			cheapestStartTime := prices.calculateCheapestHour(start, end)
			expected := time.Date(2020, 10, 10, tt.exp, 0, 0, 0, time.UTC)
			assert.Equal(t, expected, cheapestStartTime)
		})
	}
}

func TestCalculateLevel(t *testing.T) {
	t.Parallel()

	hoursPrice := []struct {
		hour  int
		price float64
	}{
		{0, 0.5},
		{1, 0.4},
		{2, 0.6},
		{3, 0.5},
		{4, 0.5},
		{5, 0.1},
		{6, 1},
		{7, 1},
		{8, 5},
		{9, 5},
		{10, 1},
		{11, 1},
		{12, 1},
		{13, 1},
		{14, 1},
		{15, 1},
		{16, 1.5},
		{17, 1.5},
		{18, 1.5},
		{19, 1.5},
		{20, 0.9},
		{21, 0.9},
		{22, 0.9},
		{23, 0.9},
		{24, 0.9},
		{25, 0.9},
		{26, 1.4},
		{27, 2.9},
	}

	prices := NewPrices()
	for _, v := range hoursPrice {
		prices.Add(newPriceTotal(v.hour, v.price))
	}

	t1 := time.Date(2020, 10, 10, 25, 0, 0, 0, time.UTC)
	_, lvl, _, _ := prices.calculateLevel(t1, 0.9)
	t.Log("level: ", lvl)
	assert.Equal(t, 1, lvl)

	t1 = time.Date(2020, 10, 10, 26, 0, 0, 0, time.UTC)
	_, lvl, _, _ = prices.calculateLevel(t1, 1.4)
	t.Log("level: ", lvl)
	assert.Equal(t, 1, lvl)

	t1 = time.Date(2020, 10, 10, 27, 0, 0, 0, time.UTC)
	_, lvl, _, _ = prices.calculateLevel(t1, 2.9)
	t.Log("level: ", lvl)
	assert.Equal(t, 3, lvl)

	t1 = time.Date(2020, 10, 10, 9, 0, 0, 0, time.UTC)
	_, lvl, _, _ = prices.calculateLevel(t1, 5.0)
	t.Log("level: ", lvl)
	assert.Equal(t, 3, lvl)

	ss := []Price{}
	for _, p := range prices.prices {
		ss = append(ss, p)
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[j].Time.After(ss[i].Time)
	})

	for _, p := range ss {
		diff, lvl, _, _ := prices.calculateLevel(p.Time, p.Total)
		t.Logf("%s price: %f lvl: %d diff: %f\n", p.Time, p.Total, lvl, diff)
	}
}
func TestCalculateLevel2(t *testing.T) {
	t.Parallel()

	hoursPrice := []struct {
		hour  int
		price float64
	}{
		{0, 0.79},
		{1, 0.704},
		{2, 0.549},
		{3, 0.693},
		{4, 0.688},
		{5, 0.697},
		{6, 0.709},
		{7, 0.713},
		{8, 0.762},
		{9, 2.0},
		{10, 2.01},
		{11, 2.11},
		{12, 2.12},
		{13, 2.11},
		{14, 2.11},
		{15, 2.11},
		{16, 1.11},
		{17, 0.02}, // this is more than 12h before 30 so should not affect level which means 30 is level 1
		{18, 1.01},
		{19, 1.01},
		{20, 1.11},
		{21, 1.10},
		{22, 1.10},
		{23, 1.12},
		{24, 1.549},
		{25, 1.693},
		{26, 1.688},
		{27, 1.697},
		{28, 1.697},
		{29, 1.697},
		{30, 0.5},
	}

	prices := NewPrices()
	for _, v := range hoursPrice {
		prices.Add(newPriceTotal(v.hour, v.price))
	}

	ss := []Price{}
	for _, p := range prices.prices {
		ss = append(ss, p)
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[j].Time.After(ss[i].Time)
	})

	t1 := time.Date(2020, 10, 11, 6, 0, 0, 0, time.UTC)
	_, lvl, _, _ := prices.calculateLevel(t1, 0.5)
	t.Log("level: ", lvl)
	assert.Equal(t, 1, lvl)

	for _, p := range ss {
		diff, lvl, _, _ := prices.calculateLevel(p.Time, p.Total)
		t.Logf("%s price: %f lvl: %d diff: %f\n", p.Time, p.Total, lvl, diff)
	}
}

func TestCheapestTimes(t *testing.T) {
	t.Parallel()

	hoursPrice := []struct {
		hour  int
		price float64
	}{
		{0, 0.5},
		{1, 0.4},
		{2, 0.6},
		{3, 0.5},
		{4, 0.5},
		{5, 0.1},
		{6, 1},
		{7, 1},
		{8, 5},
		{9, 5},
		{10, 1},
		{11, 1},
		{12, 1},
		{13, 1},
		{14, 1},
		{15, 1},
		{16, 1.5},
		{17, 1.5},
		{18, 1.5},
		{19, 1.5},
		{20, 0.9},
		{21, 0.9},
		{22, 0.9},
		{23, 0.9},
		{24, 0.9},
		{25, 0.9},
		{26, 1.4},
		{27, 2.9},
	}

	prices := NewPrices()
	for _, v := range hoursPrice {
		prices.Add(newPriceTotal(v.hour, v.price))
	}
	first := time.Date(2020, 10, 10, 0, 0, 0, 0, time.UTC)

	c := prices.calculateCheapestTimes(first, 3, 5)
	t.Log(c)
	for _, e := range c {
		t.Log(e)
	}

	assert.Equal(t, time.Date(2020, 10, 10, 5, 0, 0, 0, time.UTC), c[0])
	assert.Equal(t, time.Date(2020, 10, 10, 10, 0, 0, 0, time.UTC), c[1])
	assert.Equal(t, time.Date(2020, 10, 10, 20, 0, 0, 0, time.UTC), c[2])
}
