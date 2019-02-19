#!/bin/bash

if [ -d .repo ]; then
      find . -maxdepth 1 -type d -and -not -name '.*' -print
        read -p "Delete? (y/n) " REPLY
          if [ "$REPLY" = "y" ]; then
                  find . -maxdepth 1 -type d -and -not -name '.*' -exec rm -rf {} \;
                      repo sync
                        else
                                echo "Nothing done"
                                  fi
                              else
                                      echo "Not in a valid repo"
                                  fi
