#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module gs_utils."""

import __builtin__
import os
import sys
import tempfile
import time

# Appending to PYTHONPATH to find common.
sys.path.append(os.path.join(os.pardir, os.pardir, 'third_party',
                             'chromium_buildbot', 'scripts'))
sys.path.append(os.path.join(os.pardir, os.pardir, 'third_party',
                             'chromium_buildbot', 'site_config'))
from common import chromium_utils

from slave import slave_utils

import gs_utils
import unittest


GSUTIL_LOCATION = os.path.join(
    os.pardir, os.pardir, 'third_party', 'chromium_buildbot', 'scripts',
    'slave', 'gsutil')

TEST_TIMESTAMP = '1354128965'
TEST_TIMESTAMP_2 = '1354128985'


class TestGSUtils(unittest.TestCase):

  def setUp(self):
    self._expected_command = None
    self._test_temp_file = None
    self._test_gs_base = None
    self._test_destdir = None
    self._test_gs_acl = None

    def _MockCommand(command):
      self.assertEquals(self._expected_command, ' '.join(command))

    def _MockGSUtilFileCopy(filename, gs_base, subdir, gs_acl):
      self.assertEquals(self._test_temp_file, filename)
      self.assertEquals(self._test_gs_base, gs_base)
      self.assertEquals(self._test_destdir, subdir)
      self.assertEquals(self._test_gs_acl, gs_acl)

    def _MockGSUtilDownloadFile(src, dst):
      pass

    class _MockFile():
      def __init__(self, name, attributes):
        self._name = name

      def readlines(self):
        return []

      def read(self):
        if self._name == os.path.join(tempfile.gettempdir(), 'TIMESTAMP'):
          return TEST_TIMESTAMP
        else:
          return TEST_TIMESTAMP_2

      def close(self):
        pass

      def write(self, str):
        pass

    self._original_run_command = chromium_utils.RunCommand
    chromium_utils.RunCommand = _MockCommand

    self._original_gsutil_file_copy = slave_utils.GSUtilCopyFile
    slave_utils.GSUtilCopyFile = _MockGSUtilFileCopy
    
    self._original_gsutil_download_file = slave_utils.GSUtilDownloadFile
    slave_utils.GSUtilDownloadFile = _MockGSUtilDownloadFile

    self._original_file = __builtin__.open
    __builtin__.open = _MockFile

  def tearDown(self):
    chromium_utils.RunCommand = self._original_run_command
    slave_utils.GSUtilCopyFile = self._original_gsutil_file_copy
    slave_utils.GSUtilDownloadFile = self._original_gsutil_download_file
    __builtin__.open = self._original_file

  def test_DeleteStorageObject(self):
    self._expected_command = ('%s rm -R superman' % GSUTIL_LOCATION)
    gs_utils.DeleteStorageObject('superman')

  def testCopyStorageDirectory(self):
    self._expected_command = (
        '%s cp -a public -R superman batman' % GSUTIL_LOCATION)
    gs_utils.CopyStorageDirectory('superman', 'batman', 'public')

  def test_DoesStorageObjectExist(self):
    self._expected_command = ('%s ls superman' % GSUTIL_LOCATION)
    gs_utils.DoesStorageObjectExist('superman')

  def test_WriteCurrentTimeStamp(self):
    self._test_temp_file = os.path.join(tempfile.gettempdir(), 'TIMESTAMP')
    self._test_gs_base = 'gs://test'
    self._test_destdir = 'testdir'
    self._test_gs_acl = 'private'
    gs_utils.WriteCurrentTimeStamp(
        gs_base=self._test_gs_base, dest_dir=self._test_destdir,
        gs_acl=self._test_gs_acl)

  def test_AreTimeStampsEqual(self):
    self._test_gs_base = 'gs://test'
    self._test_destdir = 'testdir'

    local_dir = tempfile.mkdtemp()  
 
    # Will be false because the tmp directory will have no TIMESTAMP in it.
    self.assertFalse(
        gs_utils.AreTimeStampsEqual(
            local_dir=local_dir,
            gs_base=self._test_gs_base,
            gs_relative_dir=self._test_destdir))
 
    self._test_temp_file = os.path.join(local_dir, 'TIMESTAMP')
  
    # Will be false because the timestamps are different.
    self.assertFalse(
        gs_utils.AreTimeStampsEqual(
            local_dir=local_dir,
            gs_base=self._test_gs_base,
            gs_relative_dir=self._test_destdir))


if __name__ == '__main__':
  unittest.main()
