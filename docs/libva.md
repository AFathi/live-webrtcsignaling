# Info

hardware acceleration setup

# Compilation

following @see https://gist.github.com/Brainiarc7/24de2edef08866c304080504877239a3

```
#
# install in /usr/local
#

# create work dir
mkdir -p ~/Downloads/LIBVA

# CMRT
cd ~/Downloads/LIBVA && git clone git@github.com:intel/cmrt.git
cd cmrt && ./autogen.sh && ./configure && make && sudo make install

# libva
cd ~/Downloads/LIBVA && git clone git@github.com:01org/libva.git
cd libva && git checkout 2.0.0
./autogen.sh && ./configure && make && sudo make install

# intel-hybrid-driver
cd ~/Downloads/LIBVA && git clone git@github.com:01org/intel-hybrid-driver.git
cd intel-hybrid-driver && git checkout 1.0.2
./autogen.sh && ./configure && make && sudo make install

# intel-vaapi-driver
cd ~/Downloads/LIBVA && git clone git@github.com:01org/intel-vaapi-driver.git
cd intel-vaapi-driver && git checkout 2.0.0
./autogen.sh && ./configure --enable-hybrid-codec # <=== do not forget this flag
make && sudo make install

# libva-utils
cd ~/Downloads/LIBVA && git clone git@github.com:01org/libva-utils.git
cd libva-utils && git checkout 2.0.0

#
# Depending on the result of this command :
#
sudo bash -c 'cat /sys/kernel/debug/dri/128/name && cat /sys/kernel/debug/dri/129/name'
nvidia-drm dev=0000:01:00.0 unique=0000:01:00.0
i915 dev=0000:00:02.0 unique=0000:00:02.0

#
# here we have two cards
#
ls -la /dev/dri/
# total 0
# drwxr-xr-x   2 root root       120 janv. 18 12:18 ./
# drwxr-xr-x  20 root root      4340 janv. 18 12:18 ../
# crw-rw----+  1 root video 226,   0 janv. 18 12:18 card0
# crw-rw----+  1 root video 226,   1 janv. 18 12:18 card1
# crw-rw----+  1 root video 226, 128 janv. 18 12:18 renderD128
# crw-rw----+  1 root video 226, 129 janv. 18 12:18 renderD129

#
# the first one : renderD128 is an nvidia, the second one is the intel
#  we want the intel, so we need to patch  :
#diff --git a/common/va_display_drm.c b/common/va_display_drm.c
#index 4d9c656..b975ab9 100644
#--- a/common/va_display_drm.c
#+++ b/common/va_display_drm.c
#@@ -43,8 +43,8 @@ va_open_display_drm(void)
#     int i;
#
#     static const char *drm_device_paths[] = {
#-        "/dev/dri/renderD128",
#-        "/dev/dri/card0",
#+        "/dev/dri/renderD129",
#+        "/dev/dri/card1",
#         NULL
#     };

# then compile
./autogen.sh && ./configure && make && sudo make install

# testing:
cd /usr/local/bin && export LIBVA_DRIVER_NAME=i965 &&  ./vainfo
```

example outputs

```
marc@kubuntu:/usr/local/bin$ ll /usr/local/lib/dri/
total 15364
drwxr-xr-x 2 root root    4096 janv. 18 13:32 ./
drwxr-xr-x 6 root root    4096 janv. 18 13:31 ../
-rwxr-xr-x 1 root root     965 janv. 18 13:25 dummy_drv_video.la*
-rwxr-xr-x 1 root root   76936 janv. 18 13:25 dummy_drv_video.so*
-rwxr-xr-x 1 root root    1071 janv. 18 13:29 hybrid_drv_video.la*
-rwxr-xr-x 1 root root 6103160 janv. 18 13:29 hybrid_drv_video.so*
-rwxr-xr-x 1 root root     996 janv. 18 13:32 i965_drv_video.la*
-rwxr-xr-x 1 root root 9526072 janv. 18 13:32 i965_drv_video.so*

marc@kubuntu:/usr/local/bin$ export LIBVA_DRIVER_NAME=i965 &&  ./vainfo
libva info: VA-API version 1.0.0
libva info: va_getDriverName() returns 0
libva info: User requested driver 'i965'
libva info: Trying to open /usr/local/lib/dri/i965_drv_video.so
libva info: Found init function __vaDriverInit_1_0
libva info: va_openDriver() returns 0
vainfo: VA-API version: 1.0 (libva 2.0.0)
vainfo: Driver version: Intel i965 driver for Intel(R) Haswell Mobile - 2.0.0
vainfo: Supported profile and entrypoints
      VAProfileMPEG2Simple            : VAEntrypointVLD
      VAProfileMPEG2Simple            : VAEntrypointEncSlice
      VAProfileMPEG2Main              : VAEntrypointVLD
      VAProfileMPEG2Main              : VAEntrypointEncSlice
      VAProfileH264ConstrainedBaseline: VAEntrypointVLD
      VAProfileH264ConstrainedBaseline: VAEntrypointEncSlice
      VAProfileH264Main               : VAEntrypointVLD
      VAProfileH264Main               : VAEntrypointEncSlice
      VAProfileH264High               : VAEntrypointVLD
      VAProfileH264High               : VAEntrypointEncSlice
      VAProfileH264MultiviewHigh      : VAEntrypointVLD
      VAProfileH264MultiviewHigh      : VAEntrypointEncSlice
      VAProfileH264StereoHigh         : VAEntrypointVLD
      VAProfileH264StereoHigh         : VAEntrypointEncSlice
      VAProfileVC1Simple              : VAEntrypointVLD
      VAProfileVC1Main                : VAEntrypointVLD
      VAProfileVC1Advanced            : VAEntrypointVLD
      VAProfileNone                   : VAEntrypointVideoProc
      VAProfileJPEGBaseline           : VAEntrypointVLD
      VAProfileVP9Profile0            : VAEntrypointVLD
```

# tips

seems that version 1.7 & 1.8 of intel-hybrid-driver & intel-vaapi-driver doesn't compile out of the box  
```
marc@kubuntu:~/Downloads/LIBVA/intel-vaapi-driver.test$ make
Making all in debian.upstream
make[1]: Entering directory '/home/marc/Downloads/LIBVA/intel-vaapi-driver.test/debian.upstream'
  GEN      changelog
  GEN      control
  (...)

  i965_drv_video.c:1003:41: error: ‘struct <anonymous>’ has no member named ‘roi_rc_qp_delat_support’; did you mean ‘roi_rc_qp_delta_support’?
                         roi_config->bits.roi_rc_qp_delat_support = 0;
                                         ^
i965_drv_video.c:1008:41: error: ‘struct <anonymous>’ has no member named ‘roi_rc_qp_delat_support’; did you mean ‘roi_rc_qp_delta_support’?
                         roi_config->bits.roi_rc_qp_delat_support = 1;
                                         ^
(...)
make[1]: *** [all] Error 2
make[1]: Leaving directory '/home/marc/Downloads/LIBVA/intel-vaapi-driver.test/src'
Makefile:415: recipe for target 'all-recursive' failed
make: *** [all-recursive] Error 1
```

you will need to replace ‘roi_rc_qp_delat_support’ by ‘roi_rc_qp_delta_support’ in the source code (delat => delta ) ...
