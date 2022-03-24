#include <iostream>
#include <boost/filesystem.hpp>
#include "cli/extract.hpp"
#include "util/log.hpp"
#include "util/console.hpp"
#include "util/fstream.hpp"
#include "setup/version.hpp"

#include "lib.h"

bool logger::debug = false;
bool logger::quiet = false;

size_t logger::total_errors = 0;
size_t logger::total_warnings = 0;

inno_progress_callback prog_cb = 0;
inno_log_callback log_cb = 0;

extern "C" bool extract_inno(const char *path, const char *destination, inno_progress_callback callback, inno_log_callback log) {
  prog_cb = callback;
  log_cb = log;
  auto success = true;

  try {
    boost::filesystem::path installer = path;
    extract_options o;
    o.extract = true;
    o.output_dir = destination;
    // o.list = true;
    o.gog_galaxy = true;
    o.warn_unused = true;
    o.filenames.set_lowercase(true);

    o.include.push_back("Root_fs2.vp");
    o.include.push_back("sparky_fs2.vp");
    o.include.push_back("sparky_hi_fs2.vp");
    o.include.push_back("stu_fs2.vp");
    o.include.push_back("tango1_fs2.vp");
    o.include.push_back("tango2_fs2.vp");
    o.include.push_back("tango3_fs2.vp");
    o.include.push_back("warble_fs2.vp");
    o.include.push_back("smarty_fs2.vp");
    o.include.push_back("data2");
    o.include.push_back("data3");
    o.include.push_back("data\\freddocs");
    o.include.push_back("refcard.pdf");

    process_file(installer, o);
  } catch(const std::ios_base::failure & e) {
    log_cb(4, "Stream error while extracting files! Please make sure you downloaded both parts of the GOG installer.");
    log_cb(4, e.what());

		success = false;
	} catch(const format_error & e) {
    log_cb(4, e.what());
    success = false;
	} catch(const std::runtime_error & e) {
    log_cb(4, e.what());
    success = false;
	} catch(const setup::version_error &) {
		log_cb(4, "Not a supported installer! Please make sure you downloaded the offline installer, not the GOG installer.");
    success = false;
  } catch (const std::exception &exc) {
    log_cb(4, exc.what());
    success = false;
  }

  prog_cb = 0;
  log_cb = 0;
  return success;
}

logger::~logger() {
  uint8_t int_level;
  switch(level) {
    case Debug:
      int_level = 1;
      break;
    case Info:
      int_level = 2;
      break;
    case Warning:
      int_level = 3;
      total_warnings++;
      break;
    case Error:
      int_level = 4;
      total_errors++;
      break;
  }

  log_cb(int_level, buffer.str().c_str());
}

std::streambuf * warning_suppressor::set_streambuf(std::streambuf * streambuf) {
	return std::cerr.rdbuf(streambuf);
}

void warning_suppressor::flush() {
	restore();
	std::cerr << buffer.str();
	logger::total_warnings += warnings;
	logger::total_errors += errors;
}
