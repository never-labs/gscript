package modules

import (
	"fmt"
	"time"

	"github.com/never-labs/gscript/internal/runtime"

	stdtime "github.com/never-labs/gscript/internal/stdlib/time"
)

var timeTableCtor = runtime.NewSmallTableCtorN([]string{
	"year",
	"month",
	"day",
	"hour",
	"min",
	"sec",
	"nsec",
	"unix",
	"weekday",
	"yearday",
	"tz",
})

// goTimeToTable converts a Go time.Time to a GScript time table.
func goTimeToTable(t time.Time) *runtime.Table {
	// unix timestamp as float64 with nanosecond precision
	unix := float64(t.Unix()) + float64(t.Nanosecond())/1e9
	name, _ := t.Zone()
	vals := [...]runtime.Value{
		runtime.IntValue(int64(t.Year())),
		runtime.IntValue(int64(t.Month())),
		runtime.IntValue(int64(t.Day())),
		runtime.IntValue(int64(t.Hour())),
		runtime.IntValue(int64(t.Minute())),
		runtime.IntValue(int64(t.Second())),
		runtime.IntValue(int64(t.Nanosecond())),
		runtime.FloatValue(unix),
		runtime.IntValue(int64(t.Weekday())),
		runtime.IntValue(int64(t.YearDay())),
		runtime.StringValue(name),
	}
	return runtime.NewTableFromCtorNNonNilCache(&timeTableCtor, vals[:])
}

// tableToGoTime converts a GScript time table runtime.Value to a Go time.Time.
func tableToGoTime(v runtime.Value) (time.Time, error) {
	if !v.IsTable() {
		return time.Time{}, fmt.Errorf("expected time table, got %s", v.TypeName())
	}
	tbl := v.Table()

	// If we have a unix field, use it for precise reconstruction
	unixVal := tbl.RawGet(runtime.StringValue("unix"))
	if unixVal.IsFloat() || unixVal.IsInt() {
		f := unixVal.Number()
		sec := int64(f)
		nsec := int64((f - float64(sec)) * 1e9)
		return stdtime.UnixUTC(sec, nsec), nil
	}

	// Otherwise reconstruct from fields
	year := int(tbl.RawGet(runtime.StringValue("year")).Int())
	month := time.Month(tbl.RawGet(runtime.StringValue("month")).Int())
	day := int(tbl.RawGet(runtime.StringValue("day")).Int())
	hour := int(tbl.RawGet(runtime.StringValue("hour")).Int())
	min := int(tbl.RawGet(runtime.StringValue("min")).Int())
	sec := int(tbl.RawGet(runtime.StringValue("sec")).Int())
	nsec := int(tbl.RawGet(runtime.StringValue("nsec")).Int())

	return stdtime.DateUTC(year, month, day, hour, min, sec, nsec), nil
}

func timeSinceValue(v runtime.Value) (runtime.Value, error) {
	goTime, err := tableToGoTime(v)
	if err != nil {
		return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'time.since': %v", err)
	}
	return runtime.FloatValue(time.Since(goTime).Seconds()), nil
}

func contextDoneAndErr(v runtime.Value) (*runtime.Channel, runtime.Value, bool) {
	if !v.IsTable() {
		return nil, runtime.NilValue(), false
	}
	t := v.Table()
	done := t.RawGetString("done")
	if !done.IsChannel() {
		return nil, runtime.NilValue(), false
	}
	return done.Channel(), t.RawGetString("err"), true
}

func contextCancelledValue(done *runtime.Channel, errFn runtime.Value) (runtime.Value, bool) {
	_, ready, ok := done.TryRecvOK()
	if ready {
		if !ok {
			return contextErrValue(errFn), true
		}
		return runtime.StringValue("cancelled"), true
	}
	return runtime.NilValue(), false
}

func contextErrValue(errFn runtime.Value) runtime.Value {
	gf := errFn.GoFunction()
	if gf == nil || gf.Fn == nil {
		return runtime.StringValue("cancelled")
	}
	vals, err := gf.Fn(nil)
	if err != nil || len(vals) == 0 || vals[0].IsNil() {
		return runtime.StringValue("cancelled")
	}
	return vals[0]
}

// BuildTime creates the "time" standard library table.
func BuildTime() *runtime.Table {
	t := runtime.NewTable()

	set := func(name string, fn func([]runtime.Value) ([]runtime.Value, error)) {
		t.RawSet(runtime.StringValue(name), runtime.FunctionValue(&runtime.GoFunction{
			Name: "time." + name,
			Fn:   fn,
		}))
	}
	setFast1 := func(name string, fn func([]runtime.Value) ([]runtime.Value, error), fast func([]runtime.Value) (runtime.Value, error)) {
		t.RawSet(runtime.StringValue(name), runtime.FunctionValue(&runtime.GoFunction{
			Name:  "time." + name,
			Fn:    fn,
			Fast1: fast,
		}))
	}
	setFastArg1 := func(name string, fn func([]runtime.Value) ([]runtime.Value, error), fast func(runtime.Value) (runtime.Value, error)) {
		t.RawSet(runtime.StringValue(name), runtime.FunctionValue(&runtime.GoFunction{
			Name:     "time." + name,
			Fn:       fn,
			FastArg1: fast,
		}))
	}
	setFastArg2 := func(name string, fn func([]runtime.Value) ([]runtime.Value, error), fast func(runtime.Value, runtime.Value) (runtime.Value, error)) {
		t.RawSet(runtime.StringValue(name), runtime.FunctionValue(&runtime.GoFunction{
			Name:     "time." + name,
			Fn:       fn,
			FastArg2: fast,
		}))
	}
	setFastArg6 := func(name string, fn func([]runtime.Value) ([]runtime.Value, error), fast func(runtime.Value, runtime.Value, runtime.Value, runtime.Value, runtime.Value, runtime.Value) (runtime.Value, error)) {
		t.RawSet(runtime.StringValue(name), runtime.FunctionValue(&runtime.GoFunction{
			Name:     "time." + name,
			Fn:       fn,
			FastArg6: fast,
		}))
	}
	setFastArg2Ret2 := func(name string, fn func([]runtime.Value) ([]runtime.Value, error), fast func(runtime.Value, runtime.Value) (runtime.Value, runtime.Value, int, error)) {
		t.RawSet(runtime.StringValue(name), runtime.FunctionValue(&runtime.GoFunction{
			Name:         "time." + name,
			Fn:           fn,
			FastArg2Ret2: fast,
		}))
	}

	// Constants
	t.RawSet(runtime.StringValue("SECOND"), runtime.FloatValue(1.0))
	t.RawSet(runtime.StringValue("MINUTE"), runtime.FloatValue(60.0))
	t.RawSet(runtime.StringValue("HOUR"), runtime.FloatValue(3600.0))
	t.RawSet(runtime.StringValue("DAY"), runtime.FloatValue(86400.0))

	// time.now() -> time table
	setFast1("now", func(args []runtime.Value) ([]runtime.Value, error) {
		return []runtime.Value{runtime.TableValue(goTimeToTable(time.Now()))}, nil
	}, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.TableValue(goTimeToTable(time.Now())), nil
	})

	// time.sleep(seconds) -> nil
	// time.sleep(ctx, seconds) -> true, nil | false, err
	set("sleep", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'time.sleep'")
		}
		if done, errFn, ok := contextDoneAndErr(args[0]); ok {
			if len(args) < 2 {
				return nil, fmt.Errorf("bad argument #2 to 'time.sleep'")
			}
			secs := toFloat(args[1])
			if secs < 0 {
				return nil, fmt.Errorf("bad argument #2 to 'time.sleep' (non-negative duration expected)")
			}
			if errVal, cancelled := contextCancelledValue(done, errFn); cancelled {
				return []runtime.Value{runtime.BoolValue(false), errVal}, nil
			}
			timerCh := runtime.NewChannel(1)
			timer := time.AfterFunc(stdtime.DurationFromSeconds(secs), func() {
				_ = timerCh.Send(runtime.NilValue())
			})
			defer timer.Stop()
			chosen, _, recvOK, err := runtime.ChannelSelect([]runtime.ChannelSelectCase{
				{Kind: runtime.ChannelSelectRecv, Channel: timerCh},
				{Kind: runtime.ChannelSelectRecv, Channel: done},
			})
			if err != nil {
				return nil, err
			}
			if chosen == 0 && recvOK {
				return []runtime.Value{runtime.BoolValue(true), runtime.NilValue()}, nil
			}
			if !recvOK {
				return []runtime.Value{runtime.BoolValue(false), contextErrValue(errFn)}, nil
			}
			return []runtime.Value{runtime.BoolValue(false), runtime.StringValue("cancelled")}, nil
		}
		secs := toFloat(args[0])
		if secs < 0 {
			return nil, fmt.Errorf("bad argument #1 to 'time.sleep' (non-negative duration expected)")
		}
		time.Sleep(stdtime.DurationFromSeconds(secs))
		return nil, nil
	})

	// time.after(seconds) -> channel
	set("after", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'time.after'")
		}
		secs := toFloat(args[0])
		if secs < 0 {
			return nil, fmt.Errorf("bad argument #1 to 'time.after' (non-negative duration expected)")
		}
		ch := runtime.NewChannel(1)
		go func() {
			time.Sleep(stdtime.DurationFromSeconds(secs))
			_ = ch.Send(runtime.TableValue(goTimeToTable(time.Now())))
		}()
		return []runtime.Value{runtime.ChannelValue(ch)}, nil
	})

	// time.since(t) -> float seconds elapsed
	setFastArg1("since", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'time.since'")
		}
		v, err := timeSinceValue(args[0])
		return []runtime.Value{v}, err
	}, timeSinceValue)

	// time.unix(sec [, nsec]) -> time table
	set("unix", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'time.unix'")
		}
		sec := toInt(args[0])
		var nsec int64
		if len(args) >= 2 {
			nsec = toInt(args[1])
		}
		goTime := stdtime.UnixUTC(sec, nsec)
		return []runtime.Value{runtime.TableValue(goTimeToTable(goTime))}, nil
	})

	// time.format(t, layout) -> string
	timeFormat := func(tval, layoutVal runtime.Value) (runtime.Value, error) {
		goTime, err := tableToGoTime(tval)
		if err != nil {
			return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'time.format': %v", err)
		}
		return runtime.StringValue(stdtime.FormatWithStrftimeLayout(goTime, layoutVal.Str())), nil
	}
	setFastArg2("format", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'time.format'")
		}
		v, err := timeFormat(args[0], args[1])
		if err != nil {
			return nil, err
		}
		return []runtime.Value{v}, nil
	}, timeFormat)

	// time.parse(str, layout) -> time table, nil | nil, errMsg
	timeParse := func(strVal, layoutVal runtime.Value) (runtime.Value, runtime.Value, int, error) {
		str := strVal.Str()
		goTime, err := stdtime.ParseWithStrftimeLayout(str, layoutVal.Str())
		if err != nil {
			return runtime.NilValue(), runtime.StringValue(err.Error()), 2, nil
		}
		return runtime.TableValue(goTimeToTable(goTime)), runtime.NilValue(), 2, nil
	}
	setFastArg2Ret2("parse", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'time.parse'")
		}
		r0, r1, n, err := timeParse(args[0], args[1])
		if err != nil {
			return nil, err
		}
		if n == 1 {
			return []runtime.Value{r0}, nil
		}
		return []runtime.Value{r0, r1}, nil
	}, timeParse)

	// time.diff(t1, t2) -> float seconds (t2 - t1)
	set("diff", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'time.diff'")
		}
		t1, err := tableToGoTime(args[0])
		if err != nil {
			return nil, fmt.Errorf("bad argument #1 to 'time.diff': %v", err)
		}
		t2, err := tableToGoTime(args[1])
		if err != nil {
			return nil, fmt.Errorf("bad argument #2 to 'time.diff': %v", err)
		}
		diff := t2.Sub(t1).Seconds()
		return []runtime.Value{runtime.FloatValue(diff)}, nil
	})

	// time.add(t, seconds) -> time table
	set("add", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'time.add'")
		}
		goTime, err := tableToGoTime(args[0])
		if err != nil {
			return nil, fmt.Errorf("bad argument #1 to 'time.add': %v", err)
		}
		secs := toFloat(args[1])
		newTime := goTime.Add(stdtime.DurationFromSeconds(secs))
		return []runtime.Value{runtime.TableValue(goTimeToTable(newTime))}, nil
	})

	// time.date(year, month, day [, hour [, min [, sec]]]) -> time table
	timeDate6 := func(yearVal, monthVal, dayVal, hourVal, minVal, secVal runtime.Value) (runtime.Value, error) {
		year := int(toInt(yearVal))
		month := time.Month(toInt(monthVal))
		day := int(toInt(dayVal))
		hour := int(toInt(hourVal))
		min := int(toInt(minVal))
		sec := int(toInt(secVal))
		goTime := stdtime.DateUTC(year, month, day, hour, min, sec, 0)
		return runtime.TableValue(goTimeToTable(goTime)), nil
	}
	setFastArg6("date", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("bad argument to 'time.date'")
		}
		year := int(toInt(args[0]))
		month := time.Month(toInt(args[1]))
		day := int(toInt(args[2]))
		hour, min, sec := 0, 0, 0
		if len(args) >= 4 {
			hour = int(toInt(args[3]))
		}
		if len(args) >= 5 {
			min = int(toInt(args[4]))
		}
		if len(args) >= 6 {
			sec = int(toInt(args[5]))
		}
		goTime := stdtime.DateUTC(year, month, day, hour, min, sec, 0)
		return []runtime.Value{runtime.TableValue(goTimeToTable(goTime))}, nil
	}, timeDate6)

	// time.weekday(t) -> string (e.g. "Monday")
	set("weekday", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'time.weekday'")
		}
		goTime, err := tableToGoTime(args[0])
		if err != nil {
			return nil, fmt.Errorf("bad argument #1 to 'time.weekday': %v", err)
		}
		return []runtime.Value{runtime.StringValue(goTime.Weekday().String())}, nil
	})

	// time.month(t) -> string (e.g. "January")
	set("month", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'time.month'")
		}
		goTime, err := tableToGoTime(args[0])
		if err != nil {
			return nil, fmt.Errorf("bad argument #1 to 'time.month': %v", err)
		}
		return []runtime.Value{runtime.StringValue(goTime.Month().String())}, nil
	})

	// time.isBefore(t1, t2) -> bool
	set("isBefore", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'time.isBefore'")
		}
		t1, err := tableToGoTime(args[0])
		if err != nil {
			return nil, fmt.Errorf("bad argument #1 to 'time.isBefore': %v", err)
		}
		t2, err := tableToGoTime(args[1])
		if err != nil {
			return nil, fmt.Errorf("bad argument #2 to 'time.isBefore': %v", err)
		}
		return []runtime.Value{runtime.BoolValue(t1.Before(t2))}, nil
	})

	// time.isAfter(t1, t2) -> bool
	set("isAfter", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'time.isAfter'")
		}
		t1, err := tableToGoTime(args[0])
		if err != nil {
			return nil, fmt.Errorf("bad argument #1 to 'time.isAfter': %v", err)
		}
		t2, err := tableToGoTime(args[1])
		if err != nil {
			return nil, fmt.Errorf("bad argument #2 to 'time.isAfter': %v", err)
		}
		return []runtime.Value{runtime.BoolValue(t1.After(t2))}, nil
	})

	return t
}
