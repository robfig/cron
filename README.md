[![GoDoc](http://godoc.org/github.com/robfig/cron?status.png)](http://godoc.org/github.com/robfig/cron)
[![Build Status](https://travis-ci.org/robfig/cron.svg?branch=master)](https://travis-ci.org/robfig/cron)

# cron

## Upgrading to v3 (June 2019)

cron v3 is a major upgrade to the library that addresses all outstanding bugs,
feature requests, and rough edges. It is based on a merge of master which
contains various fixes to issues found over the years and the v2 branch which
contains some backwards-incompatible features like the ability to remove cron
jobs. In addition, v3 adds support for Go Modules and cleans up rough edges like
the timezone support.

New features:

- Extensible, key/value logging via an interface that complies with
  the github.com/go-logr/logr project.

- The new Chain & JobWrapper types allow you to install "interceptors" to add
  cross-cutting behavior like the following:
  - Recover any panics from jobs (activated by default)
  - Delay a job's execution if the previous run hasn't completed yet
  - Skip a job's execution if the previous run hasn't completed yet
  - Log each job's invocations
  - Notification when jobs are completed

  To avoid breaking backward compatibility, Entry.Job continues to be the value
  that was submitted, and Entry has a new WrappedJob property which is the one
  that is actually run.

It is backwards incompatible with both v1 and v2. These updates are required:

- The v1 branch accepted an optional seconds field at the beginning of the cron
  spec. This is non-standard and has led to a lot of confusion. The new default
  parser conforms to the standard as described by [the Cron wikipedia page].

  UPDATING: To retain the old behavior, construct your Cron with a custom
  parser:

      // Seconds field, required
      cron.New(cron.WithSeconds())

      // Seconds field, optional
      cron.New(
          cron.WithParser(
              cron.SecondOptional | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor))

- The Cron type now accepts functional options on construction rather than the
  ad-hoc behavior modification mechanisms before (setting a field, calling a setter).

  UPDATING: Code that sets Cron.ErrorLogger or calls Cron.SetLocation must be
  updated to provide those values on construction.

- CRON_TZ is now the recommended way to specify the timezone of a single
  schedule, which is sanctioned by the specification. The legacy "TZ=" prefix
  will continue to be supported since it is unambiguous and easy to do so.

  UPDATING: No update is required.

- By default, cron will no longer recover panics in jobs that it runs.
  Recovering can be surprising (see issue #192) and seems to be at odds with
  typical behavior of libraries. Relatedly, the `cron.WithPanicLogger` option
  has been removed to accommodate the more general JobWrapper type.

  UPDATING: To opt into panic recovery and configure the panic logger:

      cron.New(cron.WithChain(
          cron.Recover(logger),  // or use cron.DefaultLogger
      ))


### Background - Cron spec format

There are two cron spec formats in common usage:

- The "standard" cron format, described on [the Cron wikipedia page] and used by
  the cron Linux system utility.

- The cron format used by [the Quartz Scheduler], commonly used for scheduled
  jobs in Java software

[the Cron wikipedia page]: https://en.wikipedia.org/wiki/Cron
[the Quartz Scheduler]: http://www.quartz-scheduler.org/documentation/quartz-2.x/tutorials/crontrigger.html

The original version of this package included an optional "seconds" field, which
made it incompatible with both of these formats. Now, the "standard" format is
the default format accepted, and the Quartz format is opt-in.
