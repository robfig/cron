[![GoDoc](http://godoc.org/github.com/robfig/cron?status.png)](http://godoc.org/github.com/robfig/cron)
[![Build Status](https://travis-ci.org/robfig/cron.svg?branch=master)](https://travis-ci.org/robfig/cron)

# cron

## DRAFT - Upgrading to v3

cron v3 is a major upgrade to the library that addresses all outstanding bugs,
feature requests, and clarifications around usage. It is based on a merge of
master which contains various fixes to issues found over the years and the v2
branch which contains some backwards-incompatible features like removing cron
jobs. In addition, it adds support for Go Modules and cleans up rough edges like
the timezone support.

It is in development and will be considered released once a 3.0 version is
tagged. It is backwards incompatible with both the v1 and v2 branches.

Updates required:

- The v1 branch accepted an optional seconds field at the beginning of the cron
  spec. This is non-standard and has led to a lot of confusion. The new default
  parser conforms to the standard as described by [the Cron wikipedia page].

  UPDATING: To retain the old behavior, construct your Cron with a custom
  parser:

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

### Background - Cron spec format

There are two cron spec formats in common usage:

- The "standard" cron format, described on [the Cron wikipedia page] and used by
  the cron Linux system utility.

- The cron format used by [the Quartz Scheduler], commonly used for scheduled
  jobs in Java software

[the Cron wikipedia page]: https://en.wikipedia.org/wiki/Cron
[the Quartz Scheduler]: http://www.quartz-scheduler.org/documentation/quartz-2.x/tutorials/crontrigger.html

The original version of this package included an optional "seconds" field, which
made it incompatible with both of these formats. Instead, the schedule parser
has been extended to support both types.
