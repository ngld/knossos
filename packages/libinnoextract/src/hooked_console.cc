/*
 * Copyright (C) 2011-2019 Daniel Scharrer
 *
 * This software is provided 'as-is', without any express or implied
 * warranty.  In no event will the author(s) be held liable for any damages
 * arising from the use of this software.
 *
 * Permission is granted to anyone to use this software for any purpose,
 * including commercial applications, and to alter it and redistribute it
 * freely, subject to the following restrictions:
 *
 * 1. The origin of this software must not be misrepresented; you must not
 *    claim that you wrote the original software. If you use this software
 *    in a product, an acknowledgment in the product documentation would be
 *    appreciated but is not required.
 * 2. Altered source versions must be plainly marked as such, and must not be
 *    misrepresented as being the original software.
 * 3. This notice may not be removed or altered from any source distribution.
 */

#include "util/console.hpp"

#include <algorithm>
#include <cmath>
#include <signal.h>
#include <iostream>
#include <cstdlib>
#include <cstdio>
#include <cstring>

#include "configure.hpp"

#if INNOEXTRACT_HAVE_ISATTY
#include <unistd.h>
#endif

#if INNOEXTRACT_HAVE_IOCTL
#include <sys/ioctl.h>
#endif

#include <boost/date_time/posix_time/posix_time_types.hpp>
#include <boost/lexical_cast.hpp>
#include <boost/foreach.hpp>

#include "util/output.hpp"
#include "util/windows.hpp"

#include "internal.h"

static bool show_progress = true;

namespace color {

shell_command black =       { "" };
shell_command red =         { "" };
shell_command green =       { "" };
shell_command yellow =      { "" };
shell_command blue =        { "" };
shell_command magenta =     { "" };
shell_command cyan =        { "" };
shell_command white =       { "" };

shell_command dim_black =   { "" };
shell_command dim_red =     { "" };
shell_command dim_green =   { "" };
shell_command dim_yellow =  { "" };
shell_command dim_blue =    { "" };
shell_command dim_magenta = { "" };
shell_command dim_cyan =    { "" };
shell_command dim_white =   { "" };

shell_command reset =       { "" };

shell_command current = reset;

void init(is_enabled color, is_enabled progress) {}

} // namespace color

static bool progress_cleared = true;

void progress::clear(ClearMode mode) {
	progress_cleared = true;
}

void progress::show(float value, const std::string & label) {
	if(!show_progress) {
		return;
	}

  prog_cb(label.c_str(), value);
	progress_cleared = false;
}

void progress::show_unbounded(float value, const std::string & label) {
	if(!show_progress) {
		return;
	}

  prog_cb(label.c_str(), -value);
	progress_cleared = false;
}

progress::progress(boost::uint64_t max_value, bool show_value_rate)
	: max(max_value), value(0), show_rate(show_value_rate),
	  start_time(boost::posix_time::microsec_clock::universal_time()),
	  last_status(-1.f), last_time(0), last_rate(0.f) { }

bool progress::update(boost::uint64_t delta, bool force) {
	if(!show_progress) {
		return false;
	}

	force = force || progress_cleared;

	value += delta;

	float status = 0.f;
	if(max) {
		status = float(std::min(value, max)) / float(max);
		status = float(size_t(1000.f * status)) * (1.f / 1000.f);
		if(!force && status == last_status) {
			return false;
		}
	}

	boost::uint64_t time;
	try {
		boost::posix_time::ptime now(boost::posix_time::microsec_clock::universal_time());
		time = boost::uint64_t((now - start_time).total_microseconds());
	} catch(...) {
		// this shouldn't happen, assume no time has passed
		time = last_time;
	}

	#if defined(_WIN32)
	const boost::uint64_t update_interval = 100000;
	#else
	const boost::uint64_t update_interval = 50000;
	#endif
	if(!force && time - last_time < update_interval) {
		return false;
	}

	last_time = time;
	last_status = status;

	if(!max) {
		status = std::fmod(float(time) * (1.f / 5000000.f), 2.f);
		if(status > 1.f) {
			status = 2.f - status;
		}
	}

	if(show_rate) {
		if(value >= 10 * 1024 && time > 0) {
			float rate = 1000000.f * float(value) / float(time);
			if(rate != last_rate) {
				last_rate = rate;
				label.str(std::string()); // clear the buffer
				label << std::right << std::fixed << std::setfill(' ') << std::setw(5)
				      << print_bytes(rate, 1) << "/s";
			}
		}
	}

	if(max) {
		show(status, label.str());
	} else {
		show_unbounded(status, label.str());
	}

	return true;
}

void progress::set_enabled(bool enable) {
	show_progress = enable;
}

bool progress::is_enabled() {
	return show_progress;
}

