# Dependencias de terceros y sus licencias

Este proyecto se distribuye bajo [Apache License 2.0](../LICENSE).

Las siguientes dependencias se incluyen en el binario final. Cada entrada señala
la licencia de su autor; algunos paquetes del ecosistema Go usan `<SPDX>` que
puede requerir la forma "BSD 3-Clause".

| Paquete | Versión | Licencia | Copyright |
|---------|---------|----------|-----------|
| [modernc.org/sqlite](https://modernc.org/sqlite) | v1.58.0 | BSD 3-Clause | Copyright (c) 2017 The Sqlite Authors |
| [golang.org/x/sys](https://golang.org/x/sys) | v0.47.0 | BSD 3-Clause | Copyright 2009 The Go Authors |
| [modernc.org/libc](https://modernc.org/libc) | v1.75.6 | BSD 3-Clause | Copyright (c) 2017 The Libc Authors |
| [modernc.org/mathutil](https://modernc.org/mathutil) | v1.7.1 | BSD 3-Clause | Copyright (c) 2014 The mathutil Authors |
| [modernc.org/memory](https://modernc.org/memory) | v1.12.1 | BSD 3-Clause | Copyright (c) 2017 The Memory Authors |
| [github.com/dustin/go-humanize](https://github.com/dustin/go-humanize) | v1.0.1 | MIT | Copyright (c) 2005-2008 Dustin Sallings |
| [github.com/google/uuid](https://github.com/google/uuid) | v1.6.0 | BSD 3-Clause | Copyright (c) 2009,2014 Google Inc. |
| [github.com/mattn/go-isatty](https://github.com/mattn/go-isatty) | v0.0.24 | MIT | Copyright (c) Yasuhiro MATSUMOTO |
| [github.com/ncruces/go-strftime](https://github.com/ncruces/go-strftime) | v1.0.0 | MIT | Copyright (c) 2022 Nuno Cruces |
| [github.com/remyoudompheng/bigfft](https://github.com/remyoudompheng/bigfft) | v0.0.0-20230129092748-24d4a6f8daec | BSD 3-Clause | Copyright (c) 2012 The Go Authors |

## Textos de licencia

### MIT License

```
Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
```

### BSD 3-Clause License

```
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice,
   this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its contributors
   may be used to endorse or promote products derived from this software
   without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

### SQLite (dentro de modernc.org/sqlite)

SQLite es de dominio público; el código fuente ubicado en la web oficial de
SQLite no está sujeto a copyright. Esta compilación del driver la distribuye
modernc.org bajo su término de la licencia BSD 3-Clause indicado arriba.