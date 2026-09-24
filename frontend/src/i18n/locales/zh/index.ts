import desktopAuth from './desktopAuth'
import qualityOps from './qualityOps'
import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import imageWorkbench from './imageWorkbench'
import admin from './admin'
import misc from './misc'

import requestTiming from './requestTiming'

export default {
  desktopAuth,
  qualityOps,
  requestTiming,
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  ...imageWorkbench,
  admin,
  ...misc,
}
