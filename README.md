ciel-driver
============
The driver for Ciel, a manager for nspawn container.

Container
---------
// TODO

File System - overlayfs
----------------------

Do you know Photoshop?

                 TargetDir
                     ^
                     |
                == Layer == (TopLayer) <- - -> TopLayerWorkDir
                == Layer ==
                == Layer ==
                == Layer ==
                == Layer ==
                == Layer == (bottom, the "background")

("TopLayerWorkDir" is a temporary directory for an actived overlay file system.)

Any changes will stay in the top layer.
The top layer is the "difference layer", or "canvas".

You may disable/enable any layers, except for the top, essential layer.
