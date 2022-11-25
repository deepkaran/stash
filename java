
https://docs.oracle.com/en/java/javase/15/install/installation-jdk-macos.html#GUID-F9183C70-2E96-40F4-9104-F3814A5A331F

Uninstalling the JDK on macOS
To uninstall the JDK on macOS:
You must have Administrator privileges.
Note:Do not attempt to uninstall Java by removing the Java tools from /usr/bin. This directory is part of the system software and any changes will be reset by Apple the next time that you perform an update of the OS.
Go to /Library/Java/JavaVirtualMachines.
Remove the directory whose name matches the following format by executing the rm command as a root user or by using the sudo tool:
/Library/Java/JavaVirtualMachines/jdk-15.interim.update.patch.jdk
For example, to uninstall 15 Interim 0 Update 0 Patch 0:

$ rm -rf jdk-15.jdk
