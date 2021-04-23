go.notify
=========

An abstraction library for notifications. It provides following
implementations:

freedesktop
    An implementation of the `Desktop Notifications Specification`_.

gntp
    An implementation of the `Growl Notification Transport Protocol`_.

windows
    An implementation for the `Windows Notifications`_.

.. image:: https://godoc.org/github.com/hattya/go.notify?status.svg
   :target: https://godoc.org/github.com/hattya/go.notify

.. image:: https://semaphoreci.com/api/v1/hattya/go-notify/branches/master/badge.svg
   :target: https://semaphoreci.com/hattya/go-notify

.. image:: https://ci.appveyor.com/api/projects/status/ljtswx0rdyear9ft/branch/master?svg=true
   :target: https://ci.appveyor.com/project/hattya/go-notify/branch/master

.. image:: https://codecov.io/gh/hattya/go.notify/branch/master/graph/badge.svg
   :target: https://codecov.io/gh/hattya/go.notify

.. _Desktop Notifications Specification: https://developer.gnome.org/notification-spec/
.. _Growl Notification Transport Protocol: http://www.growlforwindows.com/gfw/help/gntp.aspx
.. _Windows Notifications: https://msdn.microsoft.com/en-us/library/windows/desktop/ee330740(v=vs.85).aspx


Installation
------------

.. code:: console

   $ go get -u github.com/hattya/go.notify


License
-------

go.notify is distributed under the terms of the MIT License.


Credits
-------

The `Go gopher`_ was designed by `Renne French`_.

.. image:: https://i.creativecommons.org/l/by/3.0/80x15.png
   :target: http://creativecommons.org/licenses/by/3.0/

.. _Go gopher: https://blog.golang.org/gopher
.. _Renne French: https://reneefrench.blogspot.jp/
