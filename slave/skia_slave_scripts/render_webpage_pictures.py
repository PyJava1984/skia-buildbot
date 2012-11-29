#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Runs the Skia render_pictures executable on webpage skp files.

Copies skp files from Google Storage locally and runs the render_picture
executable. The resultant images are copied back to Google Storage. This module
can be run from the command-line like this:

cd buildbot/third_party/chromium_buildbot/slave/\
Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64/build/trunk

PYTHONPATH=../../../../../chromium_trunk/tools/perf:\
../../../../scripts:\
../../../../site_config \
python ../../../../../../slave/skia_slave_scripts/render_webpage_pictures.py \
--configuration "Debug" --target_platform "" --revision 0 \
--autogen_svn_baseurl "" --make_flags "" --test_args "" --gm_args "" \
--bench_args "" --num_cores 8 --perf_output_basedir "" \
--builder_name Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64 \
--got_revision 0 --gm_image_subdir base-shuttle_ubuntu12_ati5770 \
--do_upload_results True --dest_gsbase gs://rmistry

"""

import os
import posixpath
import shutil
import sys
import tempfile

from build_step import PLAYBACK_CANNED_ACL
from slave import slave_utils
from utils import file_utils
from utils import gs_utils
from utils import sync_bucket_subdir
from utils import shell_utils

import build_step


# The device to use for render_pictures.
RENDER_PICTURES_DEVICE = 'bitmap'

SKP_TIMEOUT_MULTIPLIER = 5


class RenderWebpagePictures(build_step.BuildStep):

  def __init__(self, args, attempts=1,
               timeout=build_step.DEFAULT_TIMEOUT * SKP_TIMEOUT_MULTIPLIER,
               no_output_timeout=build_step.DEFAULT_NO_OUTPUT_TIMEOUT):
    """Constructs a RenderWebpagePictures BuildStep instance.

    args: dictionary containing arguments to this BuildStep.
    attempts: how many times to try this BuildStep before giving up.
    timeout: maximum time allowed for this BuildStep. The default value here is
             increased because there could be a lot of skps' whose images have
             to be copied over to Google Storage.
    no_output_timeout: maximum time allowed for this BuildStep to run without
        any output.
    """
    build_step.BuildStep.__init__(self, args, attempts, timeout,
                                  no_output_timeout)

    self._dest_gsbase = (self._args.get('dest_gsbase') or
                         sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)

    # Check if gm-expected exists on Google Storage.
    self._gm_expected_exists_on_storage = gs_utils.DoesStorageObjectExist(
        posixpath.join(self._dest_gsbase,
                       self._storage_playback_dirs.PlaybackGmExpectedDir()))

  def _Run(self):
    # Create the required local storage directories.
    self._CreateLocalStorageDirs()

    # Locally copy skps generated by webpages_playback from GoogleStorage.
    self._DownloadSkpsFromStorage()

    # Render pictures.
    print '\n\n=========Rendering Pictures=========\n\n'
    render_args = self._GetRenderPictureArgs(
        self._local_playback_dirs.PlaybackSkpDir(),
        self._local_playback_dirs.PlaybackGmActualDir(),
        RENDER_PICTURES_DEVICE)
    render_cmd = [self._PathToBinary('render_pictures')] + render_args
    shell_utils.Bash(render_cmd)

    if not self._gm_expected_exists_on_storage:
      # Copy images to expected directory if gm-expected has not been created in
      # Storage yet.
      print '\n\n=========Copying gm-actual to gm-expected locally=========\n\n'
      shutil.rmtree(self._local_playback_dirs.PlaybackGmExpectedDir())
      shutil.copytree(self._local_playback_dirs.PlaybackGmActualDir(),
                      self._local_playback_dirs.PlaybackGmExpectedDir())
    elif not gs_utils.AreTimeStampsEqual(
        local_dir=self._local_playback_dirs.PlaybackGmExpectedDir(),
        gs_base=self._dest_gsbase,
        gs_relative_dir=self._storage_playback_dirs.PlaybackGmExpectedDir()):
      file_utils.CreateCleanLocalDir(
          self._local_playback_dirs.PlaybackGmExpectedDir())
      # Copy expected images from Google Storage to the local directory.
      print '\n\n=======Downloading gm-expected from Google Storage=======\n\n'
      gs_utils.CopyStorageDirectory(
          src_dir=posixpath.join(
              self._dest_gsbase,
              self._storage_playback_dirs.PlaybackGmExpectedDir(),
              '*'),
          dest_dir=self._local_playback_dirs.PlaybackGmExpectedDir(),
          gs_acl=PLAYBACK_CANNED_ACL)

    if self._do_upload_results:
      # Copy actual images to Google Storage.
      print '\n\n=========Uploading gm-actual to Google Storage=========\n\n'
      gs_utils.CopyStorageDirectory(
          src_dir=self._local_playback_dirs.PlaybackGmActualDir(),
          dest_dir=posixpath.join(
              self._dest_gsbase,
              self._storage_playback_dirs.PlaybackGmActualDir()),
          gs_acl=PLAYBACK_CANNED_ACL)
      # Add a TIMESTAMP file to the gm-actual directory in Google Storage so
      # that rebaselining will be a simple directory copy from gm-actual to
      # gm-expected.
      print '\n\n=========Adding TIMESTAMP for gm-actual=========\n\n'
      gs_utils.WriteCurrentTimeStamp(
          gs_base=self._dest_gsbase,
          dest_dir=self._storage_playback_dirs.PlaybackGmActualDir(),
          gs_acl=PLAYBACK_CANNED_ACL)

      if not self._gm_expected_exists_on_storage:
        # Copy expected images to Google Storage since they do not exist yet.
        print '\n\n========Uploading gm-expected to Google Storage========\n\n'
        gs_utils.CopyStorageDirectory(
            src_dir=self._local_playback_dirs.PlaybackGmExpectedDir(),
            dest_dir=posixpath.join(
                self._dest_gsbase,
                self._storage_playback_dirs.PlaybackGmExpectedDir()),
            gs_acl=PLAYBACK_CANNED_ACL)

        # Add a TIMESTAMP file to the gm-expected directory in Google Storage so
        # we can use directory level rsync like functionality.
        print '\n\n=========Adding TIMESTAMP for gm-expected=========\n\n'
        gs_utils.WriteCurrentTimeStamp(
            gs_base=self._dest_gsbase,
            dest_dir=self._storage_playback_dirs.PlaybackGmExpectedDir(),
            gs_acl=PLAYBACK_CANNED_ACL)

  def _DownloadSkpsFromStorage(self):
    """Download Skps from Google Storage."""
    if not gs_utils.AreTimeStampsEqual(
        local_dir=self._local_playback_dirs.PlaybackSkpDir(),
        gs_base=self._dest_gsbase,
        gs_relative_dir=self._storage_playback_dirs.PlaybackSkpDir()):
      print '\n\n========Downloading skp files from Google Storage========\n\n'
      skps_source = posixpath.join(
          self._dest_gsbase, self._storage_playback_dirs.PlaybackSkpDir(), '*')
      slave_utils.GSUtilDownloadFile(
          src=skps_source, dst=self._local_playback_dirs.PlaybackSkpDir())

  def _CreateLocalStorageDirs(self):
    """Creates required local storage directories for this script."""
    if not os.path.exists(self._local_playback_dirs.PlaybackSkpDir()):
      os.makedirs(self._local_playback_dirs.PlaybackSkpDir())
    file_utils.CreateCleanLocalDir(
        self._local_playback_dirs.PlaybackGmActualDir())

  def _GetRenderPictureArgs(self, skp_dir, out_dir, config):
    """Returns the arguments to use when invoking render_pictures."""
    return [skp_dir, '--device', config,
            '--mode', 'tile', str(self.TILE_X), str(self.TILE_Y),
            '-w', out_dir]


if '__main__' == __name__:
  sys.exit(build_step.BuildStep.RunBuildStep(RenderWebpagePictures))

